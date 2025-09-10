package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"rumble/client/internal/game/assets/fonts"
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

/* ------------------------ Fonts ------------------------ */

func loadTTF(path string, size float64) font.Face {
	b, err := os.ReadFile(path)
	if err != nil {
		return basicfont.Face7x13
	}
	ft, err := opentype.Parse(b)
	if err != nil {
		return basicfont.Face7x13
	}
	f, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: size, DPI: 96, Hinting: font.HintingFull})
	if err != nil {
		return basicfont.Face7x13
	}
	return f
}

/* ------------------------ TextBox ------------------------ */

type textBox struct {
	Title     string
	Value     string
	Mask      bool
	X, Y      int
	W, H      int
	focused   bool
	cursorOn  bool
	lastBlink time.Time
	face      font.Face
}

func newTextBox(title string, x, y, w int, mask bool, face font.Face) *textBox {
	if face == nil {
		face = basicfont.Face7x13
	}
	return &textBox{
		Title: title, X: x, Y: y, W: w, H: 44,
		Mask: mask, face: face, lastBlink: time.Now(),
	}
}

func (t *textBox) rectContains(mx, my int) bool {
	return mx >= t.X && mx <= t.X+t.W && my >= t.Y && my <= t.Y+t.H
}

func (t *textBox) update() {
	// focus via click
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		t.focused = t.rectContains(mx, my)
	}
	// caret blink
	if time.Since(t.lastBlink) > 500*time.Millisecond {
		t.cursorOn = !t.cursorOn
		t.lastBlink = time.Now()
	}
	if !t.focused {
		return
	}
	// type
	for _, r := range ebiten.AppendInputChars(nil) {
		if r == '\n' || r == '\r' {
			continue
		}
		if r >= 32 {
			t.Value += string(r)
		}
	}
	// backspace (single-step to avoid “sawing”)
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(t.Value) > 0 {
		t.Value = t.Value[:len(t.Value)-1]
	}
}

func (t *textBox) drawField(dst *ebiten.Image, focus bool) {
	// Colors tuned for Warcraft-ish glass + glow
	fill := color.NRGBA{16, 22, 34, 220}     // deep navy glass
	border := color.NRGBA{120, 160, 255, 64} // subtle blue outline
	if focus {
		border = color.NRGBA{240, 196, 25, 140} // golden focus glow
	}
	// soft shadow
	ebitenutil.DrawRect(dst, float64(t.X-2), float64(t.Y+3), float64(t.W+4), float64(t.H+5), color.NRGBA{0, 0, 0, 80})

	// rounded body
	r := float32(10)
	fillRoundRect(dst, t.X, t.Y, t.W, t.H, r, fill)

	// top highlight sweep
	ebitenutil.DrawRect(dst, float64(t.X+2), float64(t.Y+2), float64(t.W-4), float64((t.H-4)/2), color.NRGBA{100, 160, 255, 18})

	// 1px rounded border (draw as outer “stroke” by layering)
	fillRoundRect(dst, t.X, t.Y, t.W, 1, r, border)                               // top
	fillRoundRect(dst, t.X, t.Y+t.H-1, t.W, 1, r, color.NRGBA{255, 255, 255, 20}) // bottom hairline
}

func (t *textBox) drawText(dst *ebiten.Image, col color.Color, placeholder string) {
	val := t.Value
	if t.Mask && val != "" {
		val = strings.Repeat("•", len(t.Value))
	}
	lineH := text.BoundString(t.face, "Hg").Dy()
	baseline := t.Y + (t.H+lineH)/2 - 2
	const padX = 14

	if val == "" && !t.focused && placeholder != "" {
		text.Draw(dst, placeholder, t.face, t.X+padX, baseline, color.NRGBA{180, 188, 210, 140})
		return
	}
	text.Draw(dst, val, t.face, t.X+padX, baseline, col)

	if t.focused && t.cursorOn {
		w := text.BoundString(t.face, val).Dx()
		text.Draw(dst, "|", t.face, t.X+padX+w, baseline, col)
	}
}

/* ------------------------ Auth UI ------------------------ */

type AuthMode int

const (
	AuthLogin AuthMode = iota
	AuthRegister
)

type AuthUI struct {
	mode      AuthMode
	user      *textBox
	pass      *textBox
	confirm   *textBox
	msg       string
	busy      bool
	apiBase   string
	onSuccess func(string)
	done      bool
	username  string

	// layout
	cardX, cardY int
	cardW, cardH int

	// focus: 0=user,1=pass,2=confirm(register only)
	focus int

	// fonts
	titleFace font.Face
	uiFace    font.Face
	inputFace font.Face

	// tappables
	btnSubmit image.Rectangle
	segLogin  image.Rectangle
	segReg    image.Rectangle

	// remember me
	remember     bool
	rememberRect image.Rectangle

	// fantasy theme for enhanced UI
	fantasyTheme *FantasyTheme
}

func NewAuthUI(apiBase string, onSuccess func(username string)) *AuthUI {
	titleFace := fonts.Title(18) // custom font for title - reduced more
	uiFace := fonts.Title(12)    // custom font for UI elements - reduced a bit more
	inpFace := fonts.Title(12)   // same size as UI for input fields

	a := &AuthUI{
		mode:      AuthLogin,
		apiBase:   apiBase,
		onSuccess: onSuccess,
		titleFace: titleFace,
		uiFace:    uiFace,
		inputFace: inpFace,
		focus:     0,
		remember:  true,
	}
	a.user = newTextBox("Username", 0, 0, 360, false, inpFace)
	a.pass = newTextBox("Password", 0, 0, 360, true, inpFace)
	a.confirm = newTextBox("Confirm", 0, 0, 360, true, inpFace)
	a.user.focused = true

	// Initialize fantasy theme for enhanced UI
	a.fantasyTheme = DefaultFantasyTheme()

	return a
}

func (a *AuthUI) setFocus(i int) {
	a.focus = i
	a.user.focused = (i == 0)
	a.pass.focused = (i == 1)
	a.confirm.focused = (i == 2 && a.mode == AuthRegister)
}

func (a *AuthUI) Update() {
	// Clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()

		// segmented control
		if ptIn(mx, my, a.segLogin) {
			a.mode = AuthLogin
			a.setFocus(0)
		} else if ptIn(mx, my, a.segReg) {
			a.mode = AuthRegister
			a.setFocus(0)
		}

		// fields
		switch {
		case a.user.rectContains(mx, my):
			a.setFocus(0)
		case a.pass.rectContains(mx, my):
			a.setFocus(1)
		case a.mode == AuthRegister && a.confirm.rectContains(mx, my):
			a.setFocus(2)
		case ptIn(mx, my, a.btnSubmit):
			a.submit()
		case ptIn(mx, my, a.rememberRect):
			a.remember = !a.remember
		}
	}

	// TAB / Shift+TAB
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		back := ebiten.IsKeyPressed(ebiten.KeyShift)
		if a.mode == AuthRegister {
			if back {
				a.setFocus((a.focus + 2) % 3)
			} else {
				a.setFocus((a.focus + 1) % 3)
			}
		} else {
			if back {
				a.setFocus((a.focus + 1) % 2)
			} else {
				a.setFocus((a.focus + 1) % 2)
			}
		}
	}

	// ENTER submits
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		a.submit()
	}

	// typing
	a.user.update()
	a.pass.update()
	if a.mode == AuthRegister {
		a.confirm.update()
	}
}

func (a *AuthUI) Draw(screen *ebiten.Image) {
	sw, sh := screen.Size()

	// Background: dark-to-darker wash
	ebitenutil.DrawRect(screen, 0, 0, float64(sw), float64(sh), color.NRGBA{8, 10, 16, 255})
	ebitenutil.DrawRect(screen, 0, float64(sh/3), float64(sw), float64(2*sh/3), color.NRGBA{6, 8, 12, 255})

	// Card size & position
	a.cardW = 600
	fieldCount := 2
	if a.mode == AuthRegister {
		fieldCount = 3
	}
	headerH := 92 // segmented + title spacing
	fieldH := 54  // each field block height
	gap := 14
	buttonH := 54
	footerH := 28
	rememberH := 28
	a.cardH = headerH + fieldCount*fieldH + (fieldCount-1)*gap + rememberH + buttonH + footerH

	a.cardX = (sw - a.cardW) / 2
	a.cardY = (sh - a.cardH) / 2

	// Card: modern glass
	drawGlassCard(screen, a.cardX, a.cardY, a.cardW, a.cardH)

	// Title
	title := "Login"
	if a.mode == AuthRegister {
		title = "Register"
	}
	titleCol := color.NRGBA{220, 230, 255, 255}
	text.Draw(screen, protocol.GameName, a.titleFace, a.cardX+24, a.cardY+40, titleCol)
	text.Draw(screen, "— "+title+" —", a.uiFace, a.cardX+24, a.cardY+66, color.NRGBA{160, 190, 255, 200})

	// Segmented (Login | Register) — rounded pill
	segW := 240
	segH := 40
	segX := a.cardX + a.cardW - segW - 24
	segY := a.cardY + 26

	// track (pill)
	fillRoundRect(screen, segX, segY, segW, segH, float32(segH/2), color.NRGBA{20, 28, 44, 210})
	a.segLogin = image.Rect(segX, segY, segX+segW/2, segY+segH)
	a.segReg = image.Rect(segX+segW/2, segY, segX+segW, segY+segH)

	// knob (selected half) — also rounded; simple overlay
	if a.mode == AuthLogin {
		fillRoundRect(screen, a.segLogin.Min.X, a.segLogin.Min.Y, a.segLogin.Dx(), a.segLogin.Dy(), float32(segH/2), color.NRGBA{44, 76, 140, 200})
	} else {
		fillRoundRect(screen, a.segReg.Min.X, a.segReg.Min.Y, a.segReg.Dx(), a.segReg.Dy(), float32(segH/2), color.NRGBA{44, 76, 140, 200})
	}
	// labels
	txtY := segY + 26
	text.Draw(screen, "Login", a.uiFace, segX+20, txtY, color.White)
	text.Draw(screen, "Register", a.uiFace, segX+segW/2+20, txtY, color.White)

	// Fields
	left := a.cardX + 24
	y := a.cardY + headerH

	a.user.X, a.user.Y = left, y+4
	a.user.W, a.user.H = a.cardW-48, 44
	a.user.drawField(screen, a.focus == 0)
	a.user.drawText(screen, color.White, "Username")

	y += fieldH
	a.pass.X, a.pass.Y = left, y+4
	a.pass.W, a.pass.H = a.cardW-48, 44
	a.pass.drawField(screen, a.focus == 1)
	a.pass.drawText(screen, color.White, "Password")

	y += fieldH
	if a.mode == AuthRegister {
		a.confirm.X, a.confirm.Y = left, y+4
		a.confirm.W, a.confirm.H = a.cardW-48, 44
		a.confirm.drawField(screen, a.focus == 2)
		a.confirm.drawText(screen, color.White, "Confirm Password")
		y += fieldH
	}

	// Remember me checkbox row
	cbSize := 18
	cbX := left
	cbY := y + (28-cbSize)/2
	a.rememberRect = image.Rect(cbX, cbY, cbX+cbSize, cbY+cbSize)
	// box + fill
	ebitenutil.DrawRect(screen, float64(cbX-1), float64(cbY-1), float64(cbSize+2), float64(cbSize+2), color.NRGBA{0, 0, 0, 80})
	ebitenutil.DrawRect(screen, float64(cbX), float64(cbY), float64(cbSize), float64(cbSize), color.NRGBA{24, 28, 40, 220})
	if a.remember {
		vector.DrawFilledRect(screen, float32(cbX+3), float32(cbY+3), float32(cbSize-6), float32(cbSize-6), color.NRGBA{240, 196, 25, 220}, false)
	}
	text.Draw(screen, "Remember me", a.uiFace, cbX+cbSize+10, cbY+cbSize-2, color.NRGBA{210, 210, 220, 255})
	y += rememberH

	// Submit button (gold, chunky)
	btnW := 260
	btnH := 48
	btnX := a.cardX + (a.cardW-btnW)/2
	btnY := a.cardY + a.cardH - footerH - btnH
	a.btnSubmit = image.Rect(btnX, btnY, btnX+btnW, btnY+btnH)
	drawGoldButton(screen, a.btnSubmit, "Submit", a.uiFace)

	// Message
	if a.busy {
		text.Draw(screen, "Working...", a.uiFace, a.cardX+24, a.cardY+a.cardH-10, color.NRGBA{220, 210, 120, 255})
	} else if a.msg != "" {
		text.Draw(screen, a.msg, a.uiFace, a.cardX+24, a.cardY+a.cardH-10, color.NRGBA{255, 160, 160, 255})
	}

	// Version display at bottom of screen
	versionText := protocol.GameName + " v" + protocol.GameVersion
	versionBounds := text.BoundString(a.uiFace, versionText)
	versionX := (sw - versionBounds.Dx()) / 2
	versionY := sh - 20
	text.Draw(screen, versionText, a.uiFace, versionX, versionY, color.NRGBA{120, 130, 140, 180})
}

/* ------------------------ Visual Helpers ------------------------ */

// fillRoundRect draws a rounded rectangle via rects + 4 corner circles (fast & AA).
func fillRoundRect(dst *ebiten.Image, x, y, w, h int, r float32, col color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	if r < 0 {
		r = 0
	}
	maxr := float32(w)
	if float32(h) < maxr {
		maxr = float32(h)
	}
	if r > maxr/2 {
		r = maxr / 2
	}

	// center band
	ebitenutil.DrawRect(dst, float64(x)+float64(r), float64(y), float64(w)-float64(2*r), float64(h), col)
	// left/right bands
	ebitenutil.DrawRect(dst, float64(x), float64(y)+float64(r), float64(r), float64(h)-float64(2*r), col)
	ebitenutil.DrawRect(dst, float64(x+w)-float64(r), float64(y)+float64(r), float64(r), float64(h)-float64(2*r), col)
	// corners
	vector.DrawFilledCircle(dst, float32(x)+r, float32(y)+r, r, col, true)
	vector.DrawFilledCircle(dst, float32(x+w)-r, float32(y)+r, r, col, true)
	vector.DrawFilledCircle(dst, float32(x)+r, float32(y+h)-r, r, col, true)
	vector.DrawFilledCircle(dst, float32(x+w)-r, float32(y+h)-r, r, col, true)
}

func ptIn(x, y int, r image.Rectangle) bool {
	return x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y
}

func drawGlassCard(dst *ebiten.Image, x, y, w, h int) {
	// shadow
	fillRoundRect(dst, x-6, y+8, w+12, h+16, 18, color.NRGBA{0, 0, 0, 90})
	// body
	fillRoundRect(dst, x, y, w, h, 18, color.NRGBA{14, 18, 28, 235})

	// sweep & border
	fillRoundRect(dst, x+3, y+3, w-6, (h-6)/3, 16, color.NRGBA{70, 110, 180, 28})
	// thin borders
	fillRoundRect(dst, x, y, w, 1, 18, color.NRGBA{120, 170, 255, 55})     // top
	fillRoundRect(dst, x, y+h-1, w, 1, 18, color.NRGBA{255, 255, 255, 20}) // bottom
	fillRoundRect(dst, x, y, 1, h, 18, color.NRGBA{120, 170, 255, 40})     // left
	fillRoundRect(dst, x+w-1, y, 1, h, 18, color.NRGBA{255, 255, 255, 18}) // right
}

// Gold button with light sheen
func drawGoldButton(dst *ebiten.Image, rct image.Rectangle, label string, face font.Face) {
	// shadow
	fillRoundRect(dst, rct.Min.X-2, rct.Min.Y+3, rct.Dx()+4, rct.Dy()+5, 14, color.NRGBA{0, 0, 0, 80})
	// base gold
	fillRoundRect(dst, rct.Min.X, rct.Min.Y, rct.Dx(), rct.Dy(), 14, color.NRGBA{210, 165, 60, 255})
	// inner sheen (top half)
	fillRoundRect(dst, rct.Min.X+3, rct.Min.Y+3, rct.Dx()-6, (rct.Dy()-6)/2, 12, color.NRGBA{255, 240, 160, 70})
	// border lines (soft)
	fillRoundRect(dst, rct.Min.X, rct.Min.Y, rct.Dx(), 1, 14, color.NRGBA{255, 240, 200, 120})         // top
	fillRoundRect(dst, rct.Min.X, rct.Min.Y+rct.Dy()-1, rct.Dx(), 1, 14, color.NRGBA{90, 60, 20, 120}) // bottom

	// centered label
	lb := text.BoundString(face, label)
	tx := rct.Min.X + (rct.Dx()-lb.Dx())/2
	ty := rct.Min.Y + (rct.Dy()+lb.Dy())/2 - 2
	text.Draw(dst, label, face, tx, ty, color.NRGBA{40, 22, 8, 255})
}

/* ------------------------ Submit + HTTP ------------------------ */

func (a *AuthUI) submit() {
	if a.busy {
		return
	}
	user := strings.TrimSpace(a.user.Value)
	pass := a.pass.Value
	if user == "" || len(pass) < 6 {
		a.msg = "Enter username and 6+ char password"
		return
	}
	if a.mode == AuthRegister && pass != a.confirm.Value {
		a.msg = "Passwords do not match"
		return
	}

	a.busy = true
	a.msg = ""

	go func() {
		defer func() { a.busy = false }()
		var err error
		var token string
		switch a.mode {
		case AuthRegister:
			err = apiRegister(a.apiBase, user, pass)
			if err == nil {
				token, err = apiLogin(a.apiBase, user, pass)
			}
		case AuthLogin:
			token, err = apiLogin(a.apiBase, user, pass)
		}
		if err != nil {
			a.msg = "Error: " + err.Error()
			return
		}
		if a.remember {
			if err := SaveToken(token); err != nil {
				a.msg = "Error: " + err.Error()
				return
			}
			if err := SaveUsername(user); err != nil {
				a.msg = "Error: " + err.Error()
				return
			}
		} else {
			ClearToken()
			ClearUsername()
		}
		SetSessionToken(token)
		a.username = strings.TrimSpace(user)
		a.done = true
		a.msg = "Success!"
		if a.onSuccess != nil {
			a.onSuccess(a.username)
		}
	}()
}

func apiBaseFromEnv(defaultBase string) string {
	if v := os.Getenv("WAR_API_BASE"); strings.TrimSpace(v) != "" {
		return v
	}
	return defaultBase
}

func apiRegister(base, user, pass string) error {
	base = apiBaseFromEnv(base)
	body := map[string]string{
		"username":         user,
		"password":         pass,
		"password_confirm": pass,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", base+"/api/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return errors.New("username already exists")
	}
	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(resp.Body)
		return errors.New(resp.Status + ": " + string(rb))
	}
	return nil
}

func apiLogin(base, user, pass string) (string, error) {
	base = apiBaseFromEnv(base)
	body := map[string]string{
		"username": user,
		"password": pass,
		"version":  protocol.GameVersion,
	}
	b, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", base+"/api/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(resp.Body)
		errMsg := resp.Status + ": " + string(rb)

		// Check for version mismatch in token
		if resp.StatusCode == http.StatusUnauthorized && strings.Contains(string(rb), "version mismatch") {
			// Clear invalid token
			ClearToken()
			ClearUsername()
			return "", errors.New("session expired due to version update: please login again")
		}

		return "", errors.New(errMsg)
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Token), nil
}

func (a *AuthUI) Done() bool       { return a.done }
func (a *AuthUI) Username() string { return a.username }
