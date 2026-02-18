// -*- mode: go; Encoding: utf-8; coding: utf-8 -*-
// Last updated: <2026/02/18 16:38:23 +0900>
//
// Ebitengine fullscreen sample
//
// Windows11 x64 25H2 + golang 1.25.7 64bit + Ebitengine 2.9.8

package main

import (
	"bytes"
	"embed"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"syscall"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

var (
	// 多重起動禁止処理用
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")

	//go:embed assets/*.png
	assets embed.FS
)

const (
	spriteCount   = 64
	moveThreshold = 5.0

	// 2x2のシートを想定（1コマのサイズ）
	frameW = 128
	frameH = 128

	isDebug = false
)

// スプライト用構造体
type Sprite struct {
	x, y          float64
	vx, vy        float64
	w, h          int
	color         color.RGBA
	angle         float64
	rotationSpeed float64
	img           *ebiten.Image
}

func NewSprite(sw, sh, iw, ih int, individualImg *ebiten.Image) *Sprite {
	quadrant := rand.Intn(4)
	baseAngle := rand.Float64()*40.0 + 25.0
	degree := baseAngle + float64(quadrant)*90.0
	radian := degree * (math.Pi / 180.0)
	speed := rand.Float64()*8.0 + 2.0

	return &Sprite{
		x:             rand.Float64() * float64(sw-iw),
		y:             rand.Float64() * float64(sh-ih),
		vx:            speed * math.Cos(radian),
		vy:            speed * math.Sin(radian),
		w:             iw,
		h:             ih,
		angle:         rand.Float64() * 2 * math.Pi,
		rotationSpeed: (rand.Float64() - 0.5) * 0.2,
		img:           individualImg,
	}
}

func (s *Sprite) Update(screenWidth, screenHeight int) {
	s.x += s.vx
	s.y += s.vy
	s.angle += s.rotationSpeed

	if s.x < 0 {
		s.vx = math.Abs(s.vx)
		s.x = 0
	} else if s.x+float64(s.w) > float64(screenWidth) {
		s.vx = -math.Abs(s.vx)
		s.x = float64(screenWidth - s.w)
	}
	if s.y < 0 {
		s.vy = math.Abs(s.vy)
		s.y = 0
	} else if s.y+float64(s.h) > float64(screenHeight) {
		s.vy = -math.Abs(s.vy)
		s.y = float64(screenHeight - s.h)
	}
}

func (s *Sprite) Draw(screen *ebiten.Image, img *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(s.w)/2, -float64(s.h)/2)
	op.GeoM.Rotate(s.angle)
	op.GeoM.Translate(s.x+float64(s.w)/2, s.y+float64(s.h)/2)
	screen.DrawImage(s.img, op)
}

// --- 以下、Game構造体とmain ---

type Game struct {
	sprites      []*Sprite
	screenWidth  int
	screenHeight int
	lastMouseX   int
	lastMouseY   int
	initialized  bool
	img          *ebiten.Image
}

func (g *Game) Update() error {
	if isDebug {
		// --- 開発時：ESCキーのみで終了 ---
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			return ebiten.Termination
		}
	} else {
		// --- 本番時：キー押し、マウス押し、マウス移動で終了 ---
		// 何らかのキーが押された
		if len(inpututil.AppendPressedKeys(nil)) > 0 {
			return ebiten.Termination
		}

		// マウスボタンが押された
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			return ebiten.Termination
		}

		// マウスが一定距離移動した
		mx, my := ebiten.CursorPosition()
		if g.initialized {
			dx, dy := float64(mx-g.lastMouseX), float64(my-g.lastMouseY)
			if math.Sqrt(dx*dx+dy*dy) > 5.0 {
				return ebiten.Termination
			}
		}
		g.lastMouseX, g.lastMouseY = mx, my
		g.initialized = true
	}

	// スプライト群を更新
	for _, s := range g.sprites {
		s.Update(g.screenWidth, g.screenHeight)
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0, 0, 0, 255})

	// スプライト群を描画
	for _, s := range g.sprites {
		s.Draw(screen, g.img)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	g.screenWidth, g.screenHeight = outsideWidth, outsideHeight
	return outsideWidth, outsideHeight
}

func main() {
	// 多重起動禁止処理
	mutexName, _ := syscall.UTF16PtrFromString("Local\\MyGopherScreenSaverMutex")

	// CreateMutexW(lpMutexAttributes, bInitialOwner, lpName)
	ret, _, err := procCreateMutex.Call(
		0,
		0,
		uintptr(unsafe.Pointer(mutexName)),
	)

	// Mutexが既に存在するかチェック
	// ERROR_ALREADY_EXISTS = 183
	if err != nil && err.(syscall.Errno) == 183 {
		// すでに起動しているので終了
		return
	}

	// Mutexのハンドルを保持し続ける必要がある（GCで消えないように）
	_ = ret

	// 内蔵された数種類の画像の中からランダムにどれかを選ぶ
	imageNumber := rand.Intn(7) + 1
	path := fmt.Sprintf("assets/gophers%d.png", imageNumber)
	imgByte, err := assets.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	// 画像のバイト列を読み出してスプライトシート画像を取得
	rawImg, _, err := image.Decode(bytes.NewReader(imgByte))
	if err != nil {
		log.Fatal("Failed to load image: ", err)
	}
	spritesheet := ebiten.NewImageFromImage(rawImg)

	// 2x2の4パターンを切り出して保持しておく
	var variants []*ebiten.Image
	for row := 0; row < 2; row++ {
		for col := 0; col < 2; col++ {
			sx, sy := col*frameW, row*frameH
			rect := image.Rect(sx, sy, sx+frameW, sy+frameH)
			variants = append(variants, spritesheet.SubImage(rect).(*ebiten.Image))
		}
	}

	// 最前面表示になることを期待
	ebiten.SetWindowFloating(true)

	var sw, sh int
	if isDebug {
		// 開発時用: ウィンドウモード時の論理サイズ
		sw, sh = 1280, 720
		ebiten.SetWindowSize(sw, sh)
		ebiten.SetFullscreen(false)
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	} else {
		// 本番用: フルスクリーン表示
		sw, sh = ebiten.Monitor().Size()
		ebiten.SetFullscreen(true)

		// マウスカーソルを非表示にする
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}

	// スプライト群を初期化
	sprites := make([]*Sprite, spriteCount)
	for i := 0; i < spriteCount; i++ {
		targetImg := variants[i%len(variants)]
		sprites[i] = NewSprite(sw, sh, frameW, frameH, targetImg)
	}

	game := &Game{
		sprites:      sprites,
		screenWidth:  sw,
		screenHeight: sh,
	}

	// メインループ開始
	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
