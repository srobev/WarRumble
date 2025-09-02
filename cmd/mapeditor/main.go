package main

import (
    "fmt"
    "encoding/json"
    "flag"
    "image/color"
    "io"
    "log"
    "net/http"
    neturl "net/url"
    "os"
    "path/filepath"
    "crypto/sha1"
    "encoding/hex"
    "regexp"
    "strings"
    "time"

    "rumble/shared/protocol"

    "github.com/gorilla/websocket"
    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/ebitenutil"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
    "github.com/hajimehoshi/ebiten/v2/text"
    "golang.org/x/image/font/basicfont"
)

type wsMsg struct {
    Type string          `json:"type"`
    Data json.RawMessage `json:"data"`
}

type editor struct {
    // connection
    ws   *websocket.Conn
    inCh chan wsMsg

    // state
    bgPath string
    bg     *ebiten.Image
    def    protocol.MapDef
    tmpLane []protocol.PointF
    tool   int // 0 deploy, 1 stone, 2 mine, 3 lane
    name   string
    status string

    // selection & editing
    selKind   string // ""|"deploy"|"stone"|"mine"|"lane"
    selIndex  int
    selHandle int // for deploy corners: 0=TL,1=TR,2=BR,3=BL, -1=body
    dragging  bool
    lastMx    int
    lastMy    int

    // bg management
    bgInput string
    bgFocus bool
    showGrid bool

    // assets browser
    assetsOpen bool
    assets     []string
    assetsSel  int
    assetsScroll int

    // name input focus
    nameFocus bool
}

func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

// ConfigDir: mimic client logic so we can reuse token.json
func sanitize(s string) string {
    s = strings.TrimSpace(strings.ToLower(s))
    s = strings.ReplaceAll(s, " ", "_")
    re := regexp.MustCompile(`[^a-z0-9._-]`)
    return re.ReplaceAllString(s, "")
}
func profileID() string {
    if p := strings.TrimSpace(os.Getenv("WAR_PROFILE")); p != "" { return sanitize(p) }
    exe, _ := os.Executable()
    base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
    h := sha1.Sum([]byte(exe))
    return sanitize(base) + "-" + hex.EncodeToString(h[:])[:8]
}
func configDir() string {
    root, _ := os.UserConfigDir()
    if root == "" { if home, _ := os.UserHomeDir(); home != "" { root = filepath.Join(home, ".config") } }
    d := filepath.Join(root, "WarRumble", profileID())
    _ = os.MkdirAll(d, 0o755)
    return d
}
func loadToken() string {
    b, _ := os.ReadFile(filepath.Join(configDir(), "token.json"))
    return strings.TrimSpace(string(b))
}

func dialWS(url, token string) (*websocket.Conn, error) {
    d := websocket.Dialer{HandshakeTimeout: 5 * time.Second,
        Proxy: func(*http.Request) (*neturl.URL, error) { return nil, nil }}
    // add token as header and query param (server accepts either)
    if token != "" {
        if u, err := neturl.Parse(url); err == nil {
            q := u.Query(); q.Set("token", token); u.RawQuery = q.Encode(); url = u.String()
        }
    }
    hdr := http.Header{}
    if token != "" { hdr.Set("Authorization", "Bearer "+token) }
    c, _, err := d.Dial(url, hdr)
    return c, err
}

func (e *editor) runReader() {
    for {
        _, data, err := e.ws.ReadMessage()
        if err != nil { close(e.inCh); return }
        var m wsMsg
        if json.Unmarshal(data, &m) == nil {
            e.inCh <- m
        }
    }
}

func (e *editor) Update() error {
    // pump messages
    for {
        select {
        case m := <-e.inCh:
            switch m.Type {
            case "MapDef":
                var md protocol.MapDefMsg
                _ = json.Unmarshal(m.Data, &md)
                e.def = md.Def
                if strings.TrimSpace(e.name) == "" { e.name = md.Def.Name }
                e.status = "Loaded"
            case "Maps":
                // ignore
            case "Error":
                var em protocol.ErrorMsg
                _ = json.Unmarshal(m.Data, &em)
                e.status = em.Message
            }
        default:
            goto done
        }
    }
done:
    mx, my := ebiten.CursorPosition()
    // toolbar
    btn := func(i int) (x,y,w,h int) { return 8, 8+i*28, 100, 24 }
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        for i := 0; i < 4; i++ {
            x,y,w,h := btn(i); if mx>=x && mx<x+w && my>=y && my<y+h { e.tool=i }
        }
    }
    // compute display rect consistent with Draw
    getDisp := func() (offX, offY, dw, dh int) {
        const topUIH = 120
        if e.bg == nil { w,h := ebiten.WindowSize(); return 0,topUIH,w,h-topUIH }
        sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
        vw, vh := ebiten.WindowSize(); vh -= topUIH
        sx := float64(vw)/float64(sw)
        sy := float64(vh)/float64(sh)
        s := sx; if sy < sx { s = sy }
        dw = int(float64(sw)*s); dh = int(float64(sh)*s)
        offX = (vw - dw)/2; offY = topUIH + (vh - dh)/2
        return offX, offY, dw, dh
    }
    offX, offY, dispW, dispH := getDisp()
    inCanvas := func(x,y int) bool { return x>=offX && x<offX+dispW && y>=offY && y<offY+dispH }
    toNorm := func(px,py int) (float64,float64) { return float64(px-offX)/float64(dispW), float64(py-offY)/float64(dispH) }
    toPix := func(nx,ny float64) (int,int) { return offX+int(nx*float64(dispW)), offY+int(ny*float64(dispH)) }

    // hit tests
    hitDeploy := func(mx,my int) (idx int, handle int, ok bool) {
        for i, r := range e.def.DeployZones {
            x,y := toPix(r.X, r.Y)
            w := int(r.W*float64(dispW)); h := int(r.H*float64(dispH))
            // corners
            corners := [][4]int{{x-4,y-4,8,8},{x+w-4,y-4,8,8},{x+w-4,y+h-4,8,8},{x-4,y+h-4,8,8}}
            for ci, c := range corners { if mx>=c[0] && mx<c[0]+c[2] && my>=c[1] && my<c[1]+c[3] { return i, ci, true } }
            if mx>=x && mx<x+w && my>=y && my<y+h { return i, -1, true }
        }
        return -1,0,false
    }
    hitPointList := func(pts []protocol.PointF, mx,my int, radius int) (int,bool) {
        r2 := radius*radius
        for i,p := range pts { px,py := toPix(p.X,p.Y); dx,dy := mx-px, my-py; if dx*dx+dy*dy <= r2 { return i,true } }
        return -1,false
    }
    distToSegSq := func(x,y, x1,y1,x2,y2 int) float64 {
        dx := float64(x2-x1); dy := float64(y2-y1)
        if dx==0 && dy==0 { ex,ey := float64(x-x1), float64(y-y1); return ex*ex+ey*ey }
        t := (float64(x-x1)*dx + float64(y-y1)*dy) / (dx*dx+dy*dy)
        if t<0 { t=0 } else if t>1 { t=1 }
        px := float64(x1)+t*dx; py := float64(y1)+t*dy
        ex,ey := float64(x)-px, float64(y)-py
        return ex*ex+ey*ey
    }
    hitLane := func(mx,my int) (int,bool) {
        th2 := 6.0*6.0
        for i, ln := range e.def.Lanes {
            for j:=1; j<len(ln.Points); j++ {
                x1,y1 := toPix(ln.Points[j-1].X, ln.Points[j-1].Y)
                x2,y2 := toPix(ln.Points[j].X, ln.Points[j].Y)
                if distToSegSq(mx,my,x1,y1,x2,y2) <= th2 { return i,true }
            }
        }
        return -1,false
    }

    // mouse press: select or create
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        e.dragging = false
        if inCanvas(mx,my) {
            if idx,h,ok := hitDeploy(mx,my); ok {
                e.selKind, e.selIndex, e.selHandle = "deploy", idx, h
                e.dragging, e.lastMx, e.lastMy = true, mx, my
            } else if i,ok := hitPointList(e.def.MeetingStones,mx,my,6); ok {
                e.selKind, e.selIndex, e.selHandle = "stone", i, -1; e.dragging=true; e.lastMx, e.lastMy = mx,my
            } else if i,ok := hitPointList(e.def.GoldMines,mx,my,6); ok {
                e.selKind, e.selIndex, e.selHandle = "mine", i, -1; e.dragging=true; e.lastMx, e.lastMy = mx,my
            } else if i,ok := hitLane(mx,my); ok {
                e.selKind, e.selIndex, e.selHandle = "lane", i, -1; e.dragging=true; e.lastMx, e.lastMy = mx,my
            } else {
                nx,ny := toNorm(mx,my)
                switch e.tool {
                case 0:
                    e.def.DeployZones = append(e.def.DeployZones, protocol.RectF{X:nx-0.05,Y:ny-0.05,W:0.1,H:0.1})
                    e.selKind, e.selIndex, e.selHandle = "deploy", len(e.def.DeployZones)-1, -1
                case 1:
                    e.def.MeetingStones = append(e.def.MeetingStones, protocol.PointF{X:nx,Y:ny})
                    e.selKind, e.selIndex = "stone", len(e.def.MeetingStones)-1
                case 2:
                    e.def.GoldMines = append(e.def.GoldMines, protocol.PointF{X:nx,Y:ny})
                    e.selKind, e.selIndex = "mine", len(e.def.GoldMines)-1
                case 3:
                    e.tmpLane = append(e.tmpLane, protocol.PointF{X:nx,Y:ny})
                    e.selKind = ""
                }
                e.lastMx, e.lastMy = mx,my
            }
        }
    }
    if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) { e.dragging=false }

    // drag
    if e.dragging {
        dx := mx - e.lastMx; dy := my - e.lastMy
        if dx!=0 || dy!=0 {
            e.lastMx, e.lastMy = mx,my
            ndx := float64(dx)/float64(dispW); ndy := float64(dy)/float64(dispH)
            switch e.selKind {
            case "deploy":
                if e.selIndex>=0 && e.selIndex<len(e.def.DeployZones) {
                    r := e.def.DeployZones[e.selIndex]
                    if e.selHandle == -1 {
                        r.X += ndx; r.Y += ndy
                    } else {
                        switch e.selHandle {
                        case 0: r.X += ndx; r.Y += ndy; r.W -= ndx; r.H -= ndy
                        case 1: r.Y += ndy; r.W += ndx; r.H -= ndy
                        case 2: r.W += ndx; r.H += ndy
                        case 3: r.X += ndx; r.W -= ndx; r.H += ndy
                        }
                        if r.W < 0.02 { r.W = 0.02 }
                        if r.H < 0.02 { r.H = 0.02 }
                    }
                    if r.X < 0 { r.X = 0 }; if r.Y < 0 { r.Y = 0 }
                    if r.X+r.W > 1 { r.X = 1 - r.W }
                    if r.Y+r.H > 1 { r.Y = 1 - r.H }
                    e.def.DeployZones[e.selIndex] = r
                }
            case "stone":
                if e.selIndex>=0 && e.selIndex<len(e.def.MeetingStones) {
                    p := e.def.MeetingStones[e.selIndex]; p.X += ndx; p.Y += ndy
                    if p.X<0 {p.X=0}; if p.Y<0 {p.Y=0}; if p.X>1{p.X=1}; if p.Y>1{p.Y=1}
                    e.def.MeetingStones[e.selIndex] = p
                }
            case "mine":
                if e.selIndex>=0 && e.selIndex<len(e.def.GoldMines) {
                    p := e.def.GoldMines[e.selIndex]; p.X += ndx; p.Y += ndy
                    if p.X<0 {p.X=0}; if p.Y<0 {p.Y=0}; if p.X>1{p.X=1}; if p.Y>1{p.Y=1}
                    e.def.GoldMines[e.selIndex] = p
                }
            case "lane":
                if e.selIndex>=0 && e.selIndex<len(e.def.Lanes) {
                    ln := e.def.Lanes[e.selIndex]
                    for i := range ln.Points { ln.Points[i].X += ndx; ln.Points[i].Y += ndy }
                    e.def.Lanes[e.selIndex] = ln
                }
            }
        }
    }

    // Right click: finalize lane if drawing; otherwise clear selection
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
        if len(e.tmpLane)>0 {
            e.def.Lanes = append(e.def.Lanes, protocol.Lane{Points: append([]protocol.PointF(nil), e.tmpLane...), Dir:1})
            e.tmpLane = nil
        } else {
            e.selKind, e.selIndex, e.selHandle = "", -1, -1
            e.dragging = false
        }
    }
    // keys: save, grid, delete selection, toggle lane dir
    for _, k := range inpututil.AppendJustPressedKeys(nil) {
        if k==ebiten.KeyS && (ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)) { e.save() }
        if k==ebiten.KeyG && !e.bgFocus && !e.nameFocus { e.showGrid = !e.showGrid }
        if k==ebiten.KeyDelete || (k==ebiten.KeyBackspace && ebiten.IsKeyPressed(ebiten.KeyShift)) {
            if !e.bgFocus && !e.nameFocus { // don't treat as delete when typing
                switch e.selKind {
                case "deploy":
                    if e.selIndex>=0 && e.selIndex<len(e.def.DeployZones) {
                        e.def.DeployZones = append(e.def.DeployZones[:e.selIndex], e.def.DeployZones[e.selIndex+1:]...)
                        e.selKind = ""; e.selIndex = -1
                    }
                case "stone":
                    if e.selIndex>=0 && e.selIndex<len(e.def.MeetingStones) {
                        e.def.MeetingStones = append(e.def.MeetingStones[:e.selIndex], e.def.MeetingStones[e.selIndex+1:]...)
                        e.selKind = ""; e.selIndex = -1
                    }
                case "mine":
                    if e.selIndex>=0 && e.selIndex<len(e.def.GoldMines) {
                        e.def.GoldMines = append(e.def.GoldMines[:e.selIndex], e.def.GoldMines[e.selIndex+1:]...)
                        e.selKind = ""; e.selIndex = -1
                    }
                case "lane":
                    if e.selIndex>=0 && e.selIndex<len(e.def.Lanes) {
                        e.def.Lanes = append(e.def.Lanes[:e.selIndex], e.def.Lanes[e.selIndex+1:]...)
                        e.selKind = ""; e.selIndex = -1
                    }
                }
            }
        }
        if k==ebiten.KeyD && !e.bgFocus && !e.nameFocus {
            if e.selKind=="lane" && e.selIndex>=0 && e.selIndex<len(e.def.Lanes) {
                if e.def.Lanes[e.selIndex].Dir>=0 { e.def.Lanes[e.selIndex].Dir = -1 } else { e.def.Lanes[e.selIndex].Dir = 1 }
            }
        }
    }

    // Assets browser interactions
    // Toggle via button hit is handled in Draw click below (same top bar cluster)
    if e.assetsOpen {
        // simple list on the right
        vw, vh := ebiten.WindowSize()
        panelX := vw - 240
        panelY := 40
        panelW := 232
        panelH := vh - panelY - 8
        rowH := 20
        maxRows := panelH / rowH
        // wheel scroll when hovering
        _, wy := ebiten.Wheel()
        if wy != 0 {
            if mx>=panelX && mx<panelX+panelW && my>=panelY && my<panelY+panelH {
                e.assetsScroll -= int(wy)
                if e.assetsScroll < 0 { e.assetsScroll = 0 }
                if len(e.assets) > maxRows {
                    maxStart := len(e.assets) - maxRows
                    if e.assetsScroll > maxStart { e.assetsScroll = maxStart }
                }
            }
        }
        if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
            if mx>=panelX && mx<panelX+panelW && my>=panelY && my<panelY+panelH {
                idx := (my - panelY) / rowH + e.assetsScroll
                if idx >= 0 && idx < len(e.assets) {
                    path := e.assets[idx]
                    if img, _, err := ebitenutil.NewImageFromFile(path); err == nil {
                        e.bg = img; e.bgPath = path; e.status = "BG loaded"; e.assetsSel = idx
                    }
                }
            }
        }
    }
    for _, r := range ebiten.AppendInputChars(nil) { if r>=32 { e.name += string(r) } }
    return nil
}

func (e *editor) save() {
    nm := strings.TrimSpace(e.name)
    if nm=="" { nm="New Map" }
    e.def.Name = nm
    if strings.TrimSpace(e.def.ID)=="" { e.def.ID = strings.ReplaceAll(strings.ToLower(nm)," ","-") }
    b,_ := json.Marshal(struct{Type string `json:"type"`; Data interface{} `json:"data"`}{Type:"SaveMap", Data: protocol.SaveMap{Def:e.def}})
    _ = e.ws.WriteMessage(websocket.TextMessage, b)
    e.status = "Saved"
}

func (e *editor) Draw(screen *ebiten.Image) {
    // Top UI bar background to prevent overlap with canvas
    const topUIH = 120
    vw, vh := ebiten.WindowSize()
    ebitenutil.DrawRect(screen, 0, 0, float64(vw), float64(topUIH), color.NRGBA{28,28,40,255})

    // BG
    if e.bg != nil {
        // simple fit top with 32px toolbar reserved
        sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
        vw, vh = ebiten.WindowSize(); vh -= topUIH
        sx := float64(vw)/float64(sw)
        sy := float64(vh)/float64(sh)
        s := sx
        if sy < sx { s = sy }
        dw := int(float64(sw)*s)
        dh := int(float64(sh)*s)
        offX := (vw - dw)/2
        offY := topUIH + (vh - dh)/2
        op := &ebiten.DrawImageOptions{}
        op.GeoM.Scale(s,s)
        op.GeoM.Translate(float64(offX), float64(offY))
        screen.DrawImage(e.bg, op)
        // overlays
        toX := func(nx float64) int { return offX + int(nx*float64(dw)) }
        toY := func(ny float64) int { return offY + int(ny*float64(dh)) }
        if e.showGrid {
            // draw a light 10% grid
            for i:=1; i<10; i++ {
                x := toX(float64(i)/10.0)
                y := toY(float64(i)/10.0)
                ebitenutil.DrawLine(screen, float64(x), float64(offY), float64(x), float64(offY+dh), color.NRGBA{60,60,70,120})
                ebitenutil.DrawLine(screen, float64(offX), float64(y), float64(offX+dw), float64(y), color.NRGBA{60,60,70,120})
            }
        }
        for i, r := range e.def.DeployZones {
            x:=toX(r.X); y:=toY(r.Y); rw:=int(r.W*float64(dw)); rh:=int(r.H*float64(dh))
            ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), float64(rh), color.NRGBA{60,150,90,90})
            if e.selKind=="deploy" && e.selIndex==i {
                // border
                ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), 1, color.NRGBA{240,196,25,255})
                ebitenutil.DrawRect(screen, float64(x), float64(y+rh-1), float64(rw), 1, color.NRGBA{240,196,25,255})
                ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(rh), color.NRGBA{240,196,25,255})
                ebitenutil.DrawRect(screen, float64(x+rw-1), float64(y), 1, float64(rh), color.NRGBA{240,196,25,255})
                // handles
                handles := [][2]int{{x,y},{x+rw,y},{x+rw,y+rh},{x,y+rh}}
                for _, h := range handles { ebitenutil.DrawRect(screen, float64(h[0]-4), float64(h[1]-4), 8, 8, color.NRGBA{240,196,25,255}) }
            }
        }
        for i, p := range e.def.MeetingStones {
            col := color.NRGBA{140,120,220,255}
            if e.selKind=="stone" && e.selIndex==i { col = color.NRGBA{240,196,25,255} }
            ebitenutil.DrawRect(screen, float64(toX(p.X)-2), float64(toY(p.Y)-2), 4, 4, col)
        }
        for i, p := range e.def.GoldMines {
            col := color.NRGBA{200,170,40,255}
            if e.selKind=="mine" && e.selIndex==i { col = color.NRGBA{240,196,25,255} }
            ebitenutil.DrawRect(screen, float64(toX(p.X)-3), float64(toY(p.Y)-3), 6, 6, col)
        }
        drawPath := func(pts []protocol.PointF, col color.NRGBA) {
            for i := 1; i < len(pts); i++ {
                x0,y0 := toX(pts[i-1].X), toY(pts[i-1].Y)
                x1,y1 := toX(pts[i].X), toY(pts[i].Y)
                ebitenutil.DrawLine(screen, float64(x0), float64(y0), float64(x1), float64(y1), col)
            }
        }
        for i, ln := range e.def.Lanes {
            col := color.NRGBA{90,160,220,255}
            if ln.Dir < 0 { col = color.NRGBA{220,110,110,255} }
            if e.selKind=="lane" && e.selIndex==i { col = color.NRGBA{240,196,25,255} }
            drawPath(ln.Points, col)
        }
        if len(e.tmpLane)>0 { drawPath(e.tmpLane, color.NRGBA{200,220,90,255}) }
    }
    // toolbar
    labels := []string{"Deploy","Stone","Mine","Lane"}
    for i, lb := range labels {
        x,y,w,h := 8, 8+i*28, 100, 24
        col := color.NRGBA{0x3a,0x3a,0x50,0xff}
        if e.tool==i { col = color.NRGBA{0x55,0x77,0xaa,0xff} }
        ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), col)
        text.Draw(screen, lb, basicfont.Face7x13, x+8, y+16, color.White)
    }
    // name + save
    // name input box (first row)
    nameLblX := 140; nameLblY := 8
    text.Draw(screen, "Name:", basicfont.Face7x13, nameLblX, nameLblY+16, color.White)
    nameX := nameLblX + 48; nameY := 8
    ebitenutil.DrawRect(screen, float64(nameX), float64(nameY), 220, 24, color.NRGBA{24,28,40,220})
    nm := e.name
    // show caret when focused
    if e.nameFocus { nm = nm + "|" }
    text.Draw(screen, nm, basicfont.Face7x13, nameX+6, nameY+16, color.White)
    saveX := nameX + 240; saveY := 8
    ebitenutil.DrawRect(screen, float64(saveX), float64(saveY), 100, 24, color.NRGBA{70,110,70,255})
    text.Draw(screen, "Save", basicfont.Face7x13, saveX+30, saveY+16, color.White)
    // Clear selection button
    clrX := saveX + 110; clrY := saveY
    ebitenutil.DrawRect(screen, float64(clrX), float64(clrY), 80, 24, color.NRGBA{90,70,70,255})
    text.Draw(screen, "Clear", basicfont.Face7x13, clrX+20, clrY+16, color.White)
    mx,my := ebiten.CursorPosition()
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        if mx>=saveX && mx<saveX+100 && my>=saveY && my<saveY+24 { e.save() }
        if mx>=clrX && mx<clrX+80 && my>=clrY && my<clrY+24 { e.selKind=""; e.selIndex=-1; e.selHandle=-1; e.tmpLane=nil; e.dragging=false }
    }
    if e.status != "" { text.Draw(screen, e.status, basicfont.Face7x13, saveX+110, saveY+16, color.NRGBA{180,220,180,255}) }

    // BG path controls: input + Load + Set + Copy (second row)
    bx := 140; by := 48
    // input box
    ebitenutil.DrawRect(screen, float64(bx), float64(by), 260, 24, color.NRGBA{24,28,40,220})
    path := e.bgInput
    if path == "" { path = e.bgPath }
    show := path
    if len(show) > 34 { show = "…" + show[len(show)-33:] }
    text.Draw(screen, "BG: "+show, basicfont.Face7x13, bx+6, by+16, color.White)
    // buttons
    btn := func(x int, label string) (int,int,int,int) {
        w := 64; h := 24; ebitenutil.DrawRect(screen, float64(x), float64(by), float64(w), float64(h), color.NRGBA{60,90,120,255}); text.Draw(screen, label, basicfont.Face7x13, x+10, by+16, color.White); return x,by,w,h }
    lx,ly,lw,lh := btn(bx+270, "Load")
    sx,sy,sw,sh := btn(bx+270+70, "Set")
    cx,cy,cw,ch := btn(bx+270+140, "Copy")
    ax,ay,aw,ah := btn(bx+270+210, "Assets")
    // focus input when clicking the box
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        // name focus toggle
        if mx>=nameX && mx<nameX+220 && my>=nameY && my<nameY+24 { e.nameFocus = true; e.bgFocus = false } else if !(mx>=bx && mx<bx+260 && my>=by && my<by+24) { if !(mx>=lx && mx<lx+lw && my>=ly && my<ly+lh) && !(mx>=sx && mx<sx+sw && my>=sy && my<sy+sh) && !(mx>=cx && mx<cx+cw && my>=cy && my<cy+ch) && !(mx>=ax && mx<ax+aw && my>=ay && my<ay+ah) { e.nameFocus = false } }
        if mx>=bx && mx<bx+260 && my>=by && my<by+24 { e.bgFocus = true } else if !(mx>=lx && mx<lx+lw && my>=ly && my<ly+lh) && !(mx>=sx && mx<sx+sw && my>=sy && my<sy+sh) && !(mx>=cx && mx<cx+cw && my>=cy && my<cy+ch) { e.bgFocus = false }
        if mx>=lx && mx<lx+lw && my>=ly && my<ly+lh { // Load
            p := strings.TrimSpace(e.bgInput)
            if p == "" { p = e.bgPath }
            if p != "" {
                if img, _, err := ebitenutil.NewImageFromFile(p); err == nil { e.bg = img; e.bgPath = p; e.status = "BG loaded" } else { e.status = "BG load failed" }
            }
        }
        if mx>=sx && mx<sx+sw && my>=sy && my<sy+sh { // Set MapDef Bg
            if e.bgPath != "" { e.def.Bg = e.bgPath; e.status = "BG set in map" }
        }
        if mx>=cx && mx<cx+cw && my>=cy && my<cy+ch { // Copy to assets/maps/<id>.*
            if e.bgPath != "" && strings.TrimSpace(e.def.ID) != "" {
                ext := filepath.Ext(e.bgPath)
                if ext == "" { ext = ".png" }
                dst := filepath.Join("assets","maps", e.def.ID+ext)
                _ = os.MkdirAll(filepath.Dir(dst), 0o755)
                if in,err := os.Open(e.bgPath); err==nil { defer in.Close(); if out,err2:=os.Create(dst); err2==nil { defer out.Close(); if _,err3 := io.Copy(out,in); err3==nil { e.status = "BG copied to "+dst } } }
            }
        }
        if mx>=ax && mx<ax+aw && my>=ay && my<ay+ah { // Toggle assets list and refresh
            e.assetsOpen = !e.assetsOpen
            if e.assetsOpen {
                // scan assets/maps for images
                var out []string
                _ = filepath.Walk("assets/maps", func(path string, info os.FileInfo, err error) error {
                    if err != nil || info == nil || info.IsDir() { return nil }
                    low := strings.ToLower(path)
                    if strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") {
                        out = append(out, path)
                    }
                    return nil
                })
                e.assets = out
                e.assetsScroll = 0
                e.assetsSel = -1
            }
        }
    }
    if e.bgFocus {
        for _, k := range inpututil.AppendJustPressedKeys(nil) {
            if k==ebiten.KeyBackspace && len(e.bgInput)>0 { e.bgInput = e.bgInput[:len(e.bgInput)-1] }
            if k==ebiten.KeyEnter {
                p := strings.TrimSpace(e.bgInput)
                if p == "" { p = e.bgPath }
                if p != "" {
                    if img, _, err := ebitenutil.NewImageFromFile(p); err == nil { e.bg = img; e.bgPath = p; e.status = "BG loaded" } else { e.status = "BG load failed" }
                }
            }
        }
        for _, r := range ebiten.AppendInputChars(nil) { if r>=32 { e.bgInput += string(r) } }
    }

    if e.nameFocus {
        for _, k := range inpututil.AppendJustPressedKeys(nil) {
            if k==ebiten.KeyBackspace && len(e.name)>0 { e.name = e.name[:len(e.name)-1] }
        }
        for _, r := range ebiten.AppendInputChars(nil) { if r>=32 { e.name += string(r) } }
    }

    // Show live normalized mouse coordinates (below toolbar)
    if e.bg != nil {
        const topUIH = 120
        vw, vh := ebiten.WindowSize()
        sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
        sx := float64(vw)/float64(sw); sy := float64(vh-topUIH)/float64(sh)
        s := sx; if sy < sx { s = sy }
        dw := int(float64(sw)*s); dh := int(float64(sh)*s)
        offX := (vw - dw)/2; offY := topUIH + (vh-topUIH - dh)/2
        if mx>=offX && mx<offX+dw && my>=offY && my<offY+dh {
            nx := (float64(mx-offX))/float64(dw); ny := (float64(my-offY))/float64(dh)
            text.Draw(screen, fmt.Sprintf("(%.3f, %.3f)", nx, ny), basicfont.Face7x13, 8, 8+4*28, color.NRGBA{200,200,210,255})
        }
    }

    // Assets browser panel (draw)
    if e.assetsOpen {
        vw, vh := ebiten.WindowSize()
        panelX := vw - 260
        panelY := by + 32
        panelW := 252
        panelH := vh - panelY - 8
        ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.NRGBA{30,30,40,220})
        text.Draw(screen, "assets/maps", basicfont.Face7x13, panelX+8, panelY+16, color.White)
        rowH := 20
        maxRows := (panelH-28)/rowH
        start := e.assetsScroll
        for i := 0; i < maxRows && start+i < len(e.assets); i++ {
            yy := panelY + 24 + i*rowH
            if (start+i)%2 == 0 {
                ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{40,40,56,255})
            }
            p := e.assets[start+i]
            show := p
            if len(show) > 32 { show = "…"+show[len(show)-31:] }
            var acol color.Color = color.White
            if e.assetsSel == start+i { acol = color.NRGBA{240,196,25,255} }
            text.Draw(screen, show, basicfont.Face7x13, panelX+8, yy, acol)
        }
    }
}

func (e *editor) Layout(outsideWidth, outsideHeight int) (int, int) {
    // Match the window size so UI scales with resize
    return ebiten.WindowSize()
}

func main() {
    var wsURL, bg, mapID, flagToken string
    flag.StringVar(&wsURL, "ws", getenv("WAR_WS_URL", "ws://127.0.0.1:8080/ws"), "WebSocket URL")
    flag.StringVar(&bg, "bg", "", "Background image path (optional)")
    flag.StringVar(&mapID, "id", "", "Load existing map by ID (optional)")
    flag.StringVar(&flagToken, "token", "", "Bearer token for auth (optional)")
    flag.Parse()
    log.SetFlags(0)
    tok := strings.TrimSpace(flagToken)
    if tok == "" { tok = strings.TrimSpace(getenv("WAR_TOKEN", "")) }
    if tok == "" { tok = loadToken() }
    if tok == "" {
        log.Println("No auth token found. Provide --token, set WAR_TOKEN, or log in via the game client with the same WAR_PROFILE to share token.json.")
    }
    c, err := dialWS(wsURL, tok)
    if err != nil { log.Fatal(err) }
    ed := &editor{ws:c, inCh: make(chan wsMsg, 128)}
    go ed.runReader()
    // load initial bg if given, else attempt assets/maps/default.png
    try := func(p string){ if ed.bg!=nil || p==""{return}; if img,_,err:=ebitenutil.NewImageFromFile(p); err==nil{ ed.bg=img; ed.bgPath=p } }
    try(bg)
    try(filepath.Join("assets","maps","default.png"))
    if ed.bg == nil { // fallback: empty image
        ed.bg = ebiten.NewImage(800, 600)
        ed.bg.Fill(color.NRGBA{0x22,0x22,0x33,0xff})
    }
    // Optionally load a map by ID
    if strings.TrimSpace(mapID) != "" {
        b,_ := json.Marshal(struct{Type string `json:"type"`; Data interface{} `json:"data"`}{Type:"GetMap", Data: protocol.GetMap{ID: mapID}})
        _ = ed.ws.WriteMessage(websocket.TextMessage, b)
    }

    ebiten.SetWindowTitle("Rumble Map Editor")
    ebiten.SetWindowSize(1200, 800)
    ebiten.SetWindowResizable(true)
    if err := ebiten.RunGame(ed); err != nil { log.Fatal(err) }
}
