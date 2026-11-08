// Command mkicon draws resources/icon.png: a macOS squircle in Go blue,
// with a branch-and-merge mark that stays readable on the light cyan.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

const size = 1024

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	white, holes := gitMark()
	shadow := shift(blur(white, 26), 0, 20)
	rim := blur(white, 10)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			cov := squircleCoverage(float64(x)+0.5, float64(y)+0.5)
			if cov <= 0 {
				continue
			}
			i := y*size + x
			r, g, b := gradient(float64(x)/size, float64(y)/size)
			r, g, b = gloss(r, g, b, float64(x)/size, float64(y)/size)
			sh := float64(shadow[i]) / 255
			r, g, b = mix(r, g, b, 0.02, 0.10, 0.18, sh*0.5)
			// A slim dark edge keeps the diamond visible on the light cyan.
			if white[i] == 0 && rim[i] > 18 {
				r, g, b = mix(r, g, b, 0.03, 0.16, 0.26, 0.9)
			}
			if white[i] > 0 {
				r, g, b = mix(r, g, b, 1, 1, 1, float64(white[i])/255)
			}
			// The branch itself, the same three-node mark as the Git logo.
			if holes[i] > 0 {
				r, g, b = mix(r, g, b, 0.04, 0.14, 0.22, float64(holes[i])/255)
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: byte(clamp01(r) * cov * 255),
				G: byte(clamp01(g) * cov * 255),
				B: byte(clamp01(b) * cov * 255),
				A: byte(clamp01(cov) * 255),
			})
		}
	}
	out := filepath.Join("resources", "icon.png")
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func squircleCoverage(x, y float64) float64 {
	const a = 430.0
	const n = 5.0
	dx, dy := math.Abs(x-size/2), math.Abs(y-size/2)
	nx, ny := dx/a, dy/a
	f := math.Pow(nx, n) + math.Pow(ny, n) - 1
	dfdx := n * math.Pow(nx, n-1) / a
	dfdy := n * math.Pow(ny, n-1) / a
	g := math.Hypot(dfdx, dfdy)
	dist := a
	if g > 1e-8 {
		dist = f / g
	} else if f < 0 {
		dist = -a
	}
	return clamp01(0.5 - dist)
}

func gradient(x, y float64) (r, g, b float64) {
	// Go cyan #00ADD8, lighter at the top and deeper blue toward the bottom.
	tl := [3]float64{0.42, 0.86, 0.97}
	tr := [3]float64{0.18, 0.78, 0.93}
	bl := [3]float64{0.00, 0.42, 0.68}
	br := [3]float64{0.00, 0.55, 0.78}
	t := smooth(x)
	top := lerp3(tl, tr, t)
	bot := lerp3(bl, br, t)
	return unpack(lerp3(top, bot, smooth(y)))
}

func gloss(r, g, b, x, y float64) (float64, float64, float64) {
	// Soft light in the upper area, the same sheen as a macOS icon.
	shine := clamp01(1 - (y-0.08)/0.42)
	shine *= 0.22 * (0.65 + 0.35*math.Sin(x*math.Pi))
	return mix(r, g, b, 1, 1, 1, shine)
}

// gitMark is the Git icon: a diamond with the three-node branch cut out of it.
// The path is Jason Long's mark (CC BY 3.0), redrawn here in our colors.
func gitMark() (white, holes []uint8) {
	svg := `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="114.8625" height="114.8625">
  <path fill="#fff" d="M112.692875 52.3185 62.544 2.171625c-2.88675-2.88875-7.570875-2.88875-10.4615 0l-10.412625 10.4145 13.209 13.2095C57.94925 24.759 61.46975 25.45475 63.916 27.9015c2.461 2.462 3.150375 6.01275 2.087375 9.09375l12.732 12.7305c3.081-1.062 6.63325-.3755 9.09425 2.088875 3.4375 3.4365 3.4375 9.007375 0 12.44675-3.44 3.4395-9.00975 3.4395-12.45125 0-2.585375-2.587875-3.225-6.387125-1.914-9.57275l-11.875-11.874V74.06075c.837375.415 1.628375.96775 2.326625 1.664 3.4375 3.437125 3.4375 9.007375 0 12.44975-3.4375 3.436-9.01125 3.436-12.44625 0-3.4375-3.442375-3.4375-9.012625 0-12.44975.849625-.848625 1.8335-1.490625 2.88325-1.920375V42.26925c-1.04975-.42975-2.03125-1.066375-2.88325-1.920875-2.6035-2.602625-3.23-6.424375-1.8945-9.622125l-13.0225-13.024375-34.387225 34.38425c-2.88818125 2.891125-2.88818125 7.57525 0 10.463875l50.15135 50.146975c2.88725 2.88818125 7.569875 2.88818125 10.4615 0l49.9145-49.9146c2.889125-2.889125 2.889125-7.575625 0-10.463875"/>
</svg>`
	dir, err := os.MkdirTemp("", "gg-icon")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	svgPath := filepath.Join(dir, "mark.svg")
	pngPath := filepath.Join(dir, "mark.png")
	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		panic(err)
	}
	cmd := exec.Command("rsvg-convert", "-w", "900", "-h", "900", svgPath, "-o", pngPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("rsvg-convert: %v\n%s", err, out))
	}
	f, err := os.Open(pngPath)
	if err != nil {
		panic(err)
	}
	sprite, err := png.Decode(f)
	f.Close()
	if err != nil {
		panic(err)
	}
	const mark = 760
	origin := (size - mark) / 2
	sw, sh := sprite.Bounds().Dx(), sprite.Bounds().Dy()
	white = make([]uint8, size*size)
	for y := 0; y < mark; y++ {
		for x := 0; x < mark; x++ {
			sx := int(float64(x) * float64(sw) / float64(mark))
			sy := int(float64(y) * float64(sh) / float64(mark))
			_, _, _, a := sprite.At(sx, sy).RGBA()
			if a > 0x8000 {
				white[(origin+y)*size+(origin+x)] = 255
			}
		}
	}
	holes = make([]uint8, size*size)
	seen := make([]bool, size*size)
	q := make([]int, 0, size)
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= size || y >= size {
			return
		}
		i := y*size + x
		if seen[i] || white[i] != 0 {
			return
		}
		seen[i] = true
		q = append(q, i)
	}
	for x := 0; x < size; x++ {
		push(x, 0)
		push(x, size-1)
	}
	for y := 0; y < size; y++ {
		push(0, y)
		push(size-1, y)
	}
	for head := 0; head < len(q); head++ {
		i := q[head]
		x, y := i%size, i/size
		push(x+1, y)
		push(x-1, y)
		push(x, y+1)
		push(x, y-1)
	}
	for i, v := range white {
		if v == 0 && !seen[i] {
			holes[i] = 255
		}
	}
	return white, holes
}

func blur(src []uint8, radius int) []uint8 {
	if radius < 1 {
		return append([]uint8(nil), src...)
	}
	tmp := make([]uint8, len(src))
	dst := make([]uint8, len(src))
	box(src, tmp, radius, true)
	box(tmp, dst, radius, false)
	box(dst, tmp, radius, true)
	box(tmp, dst, radius, false)
	return dst
}

func box(src, dst []uint8, radius int, horizontal bool) {
	w := radius*2 + 1
	if horizontal {
		for y := 0; y < size; y++ {
			sum := 0
			for i := -radius; i <= radius; i++ {
				sum += int(src[y*size+clampi(i, 0, size-1)])
			}
			for x := 0; x < size; x++ {
				dst[y*size+x] = uint8(sum / w)
				sub := clampi(x-radius, 0, size-1)
				add := clampi(x+radius+1, 0, size-1)
				sum += int(src[y*size+add]) - int(src[y*size+sub])
			}
		}
		return
	}
	for x := 0; x < size; x++ {
		sum := 0
		for i := -radius; i <= radius; i++ {
			sum += int(src[clampi(i, 0, size-1)*size+x])
		}
		for y := 0; y < size; y++ {
			dst[y*size+x] = uint8(sum / w)
			sub := clampi(y-radius, 0, size-1)
			add := clampi(y+radius+1, 0, size-1)
			sum += int(src[add*size+x]) - int(src[sub*size+x])
		}
	}
}

func shift(src []uint8, dx, dy int) []uint8 {
	dst := make([]uint8, len(src))
	for y := 0; y < size; y++ {
		sy := y - dy
		if sy < 0 || sy >= size {
			continue
		}
		for x := 0; x < size; x++ {
			sx := x - dx
			if sx < 0 || sx >= size {
				continue
			}
			dst[y*size+x] = src[sy*size+sx]
		}
	}
	return dst
}

func lerp3(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
	}
}

func unpack(c [3]float64) (float64, float64, float64) { return c[0], c[1], c[2] }

func mix(r, g, b, tr, tg, tb, a float64) (float64, float64, float64) {
	return r + (tr-r)*a, g + (tg-g)*a, b + (tb-b)*a
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
