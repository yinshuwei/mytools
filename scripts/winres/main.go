// Command winres writes the Windows resource object that go build links into
// the executable: the app's icon, at the id MyGo's Windows backend loads it
// by (LoadIconW, id 1) for the window class, the taskbar button and the tray.
//
// Without it the executable carries no icon at all, and Explorer, the taskbar
// and the title bar fall back to the default program icon. mygo build writes
// such an object itself, with the manifest and the version information beside
// the icon; the scripts here call plain go build, which links only a .syso
// that is already in the package, so they run this first and remove what it
// wrote when the build is done.
//
//	go run ./scripts/winres                       both Windows arches
//	go run ./scripts/winres -o cmd/mytools        somewhere else
//	go run ./scripts/winres -icon res/icon.png    another icon
//
// It prints the path of every file it writes, one per line, so that a script
// can remove them.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"

	xdraw "golang.org/x/image/draw"
)

// iconSizes are the sizes Windows asks for: the small ones for the title bar
// and the taskbar, the large ones for the desktop, Alt+Tab and the file
// properties.
var iconSizes = []int{16, 24, 32, 48, 64, 128, 256}

// iconLang is the language the resources are stored under, English (United
// States). Windows falls back to it when it finds nothing in the language of
// the system, and there is nothing else here to find.
const iconLang = 0x0409

// The resource types the object holds: the icon images, and the group that
// names them, which is what LoadIcon reads.
const (
	rtIcon      = 3
	rtGroupIcon = 14
)

func main() {
	icon := flag.String("icon", filepath.Join("resources", "icon.png"), "square PNG to embed")
	dir := flag.String("o", filepath.Join("cmd", "mytools"), "directory of the main package the .syso goes in")
	flag.Parse()

	src, err := os.ReadFile(*icon)
	if err != nil {
		fatal(err)
	}
	images, err := iconImages(src)
	if err != nil {
		fatal(fmt.Errorf("%s: %w", *icon, err))
	}
	// Both Windows targets of the repository: the name of the file says
	// which build it belongs to, so the other one is not linked in.
	for _, arch := range []string{"amd64", "arm64"} {
		obj, err := syso(images, arch)
		if err != nil {
			fatal(err)
		}
		path := filepath.Join(*dir, "winres_windows_"+arch+".syso")
		if err := os.WriteFile(path, obj, 0o644); err != nil {
			fatal(err)
		}
		fmt.Println(path)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "winres:", err)
	os.Exit(1)
}

// iconImages renders src at each of iconSizes, as PNG: Windows takes a PNG
// for an icon image since Vista, which is smaller than the old bitmaps and
// keeps the transparency.
func iconImages(src []byte) ([][]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("must be a PNG: %w", err)
	}
	if b := img.Bounds(); b.Dx() != b.Dy() {
		return nil, fmt.Errorf("must be square, got %dx%d", b.Dx(), b.Dy())
	}
	images := make([][]byte, len(iconSizes))
	for i, size := range iconSizes {
		var p bytes.Buffer
		if err := png.Encode(&p, resize(img, size)); err != nil {
			return nil, err
		}
		images[i] = p.Bytes()
	}
	return images, nil
}

// resize scales img to a size x size square, averaging the pixels when it
// shrinks them, which keeps a small icon from turning to noise.
func resize(img image.Image, size int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}

// syso encodes the resources of one GOARCH as a COFF object, the one file
// type the Go linker reads resources from: a section of .rsrc, whose data
// entries hold the address of their data, spelled as relocations that the
// linker fills in as it lays the section out.
func syso(images [][]byte, arch string) ([]byte, error) {
	archInfo, ok := map[string]struct{ machine, relocation uint16 }{
		"amd64": {0x8664, 0x0003}, // IMAGE_REL_AMD64_ADDR32NB
		"arm64": {0xAA64, 0x0002}, // IMAGE_REL_ARM64_ADDR32NB
		"386":   {0x014C, 0x0007}, // IMAGE_REL_I386_DIR32NB
	}[arch]
	if !ok {
		return nil, fmt.Errorf("no icon resources for %s", arch)
	}

	icons := map[uint16][]byte{}
	var group bytes.Buffer
	_ = binary.Write(&group, binary.LittleEndian, [3]uint16{0, 1, uint16(len(images))})
	for i, img := range images {
		id := uint16(i + 1)
		icons[id] = img
		dim := byte(iconSizes[i])
		if iconSizes[i] >= 256 {
			dim = 0 // 256 does not fit in a byte, and is written as 0.
		}
		group.Write([]byte{dim, dim, 0, 0})
		_ = binary.Write(&group, binary.LittleEndian, [2]uint16{1, 32}) // one plane, 32 bits
		_ = binary.Write(&group, binary.LittleEndian, [2]uint32{uint32(len(img)), uint32(id)})
	}
	res := map[uint16]map[uint16][]byte{
		rtIcon:      icons,
		rtGroupIcon: {1: group.Bytes()},
	}

	// Three levels of directory, one per type, id and language, then the
	// data entries, then the data. Directory entries point at
	// subdirectories by offset in the section; data entries hold an address,
	// relative to the image, which the linker computes.
	types := sortedKeys(res)
	dirSize := func(n int) int { return 16 + 8*n }
	size := dirSize(len(types))
	entries := 0
	for _, t := range types {
		size += dirSize(len(res[t])) + len(res[t])*dirSize(1)
		entries += len(res[t])
	}
	entriesAt := size
	dataAt := entriesAt + 16*entries

	raw := make([]byte, dataAt)
	le := binary.LittleEndian
	dir := func(at, n int) {
		le.PutUint16(raw[at+14:], uint16(n)) // NumberOfIdEntries; none are named
	}
	var relocs []uint32
	var data bytes.Buffer
	dir(0, len(types))
	next := dirSize(len(types))
	entry := entriesAt
	for i, t := range types {
		le.PutUint32(raw[16+8*i:], uint32(t))
		le.PutUint32(raw[16+8*i+4:], uint32(next)|0x80000000) // a subdirectory
		ids := sortedKeys(res[t])
		idDir := next
		dir(idDir, len(ids))
		next += dirSize(len(ids))
		for j, id := range ids {
			le.PutUint32(raw[idDir+16+8*j:], uint32(id))
			le.PutUint32(raw[idDir+16+8*j+4:], uint32(next)|0x80000000)
			dir(next, 1)
			le.PutUint32(raw[next+16:], iconLang)
			le.PutUint32(raw[next+20:], uint32(entry))
			next += dirSize(1)

			// The data of an entry is 8-byte aligned, as Windows expects.
			for data.Len()%8 != 0 {
				data.WriteByte(0)
			}
			le.PutUint32(raw[entry:], uint32(dataAt+data.Len())) // OffsetToData
			le.PutUint32(raw[entry+4:], uint32(len(res[t][id])))
			relocs = append(relocs, uint32(entry))
			entry += 16
			data.Write(res[t][id])
		}
	}
	raw = append(raw, data.Bytes()...)
	for len(raw)%4 != 0 {
		raw = append(raw, 0)
	}

	const headerSize, sectionHeaderSize = 20, 40
	rawAt := headerSize + sectionHeaderSize
	relocAt := rawAt + len(raw)
	symbolsAt := relocAt + 10*len(relocs)

	var out bytes.Buffer
	_ = binary.Write(&out, le, struct {
		Machine, Sections                   uint16
		Time, Symbols, SymbolCount          uint32
		OptionalHeaderSize, Characteristics uint16
	}{archInfo.machine, 1, 0, uint32(symbolsAt), 1, 0, 0})
	_ = binary.Write(&out, le, struct {
		Name                                        [8]byte
		VirtualSize, VirtualAddress, RawSize, RawAt uint32
		RelocAt, LineNumbersAt                      uint32
		Relocs, LineNumbers                         uint16
		Characteristics                             uint32
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 0, uint32(len(raw)), uint32(rawAt), uint32(relocAt), 0,
		uint16(len(relocs)), 0, 0x40000040}) // initialized data, readable
	out.Write(raw)
	for _, r := range relocs {
		_ = binary.Write(&out, le, r)         // VirtualAddress
		_ = binary.Write(&out, le, uint32(0)) // symbol 0: the section
		_ = binary.Write(&out, le, archInfo.relocation)
	}
	// The section symbol, then an empty string table.
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	_ = binary.Write(&out, le, uint32(0)) // Value
	_ = binary.Write(&out, le, uint16(1)) // SectionNumber
	_ = binary.Write(&out, le, uint16(0)) // Type
	out.Write([]byte{3, 0})               // IMAGE_SYM_CLASS_STATIC, no aux symbols
	_ = binary.Write(&out, le, uint32(4)) // the empty string table
	return out.Bytes(), nil
}

func sortedKeys[V any](m map[uint16]V) []uint16 {
	keys := make([]uint16, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
