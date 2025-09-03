package game

func (g *Game) pvpLayout() (queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn rect) {
	const fieldW, fieldH = 180, 24
	const btnH = 28

	x := pad
	y := topBarH + pad + 22 + 60 // Added 60 pixels for the title area

	queueBtn = rect{x: x, y: y, w: 150, h: btnH}
	leaveBtn = rect{x: x, y: y, w: 150, h: btnH} // Same position as queueBtn

	createBtn = rect{x: x, y: y + 44, w: 220, h: btnH}
	cancelBtn = rect{x: x, y: y + 44, w: 180, h: btnH} // Same position as createBtn, wider for "Cancel Friendly"

	joinY := y + 100
	if g.pvpHosting && g.pvpCode != "" {
		codeBottom := (createBtn.y + btnH + 12) + 26
		want := codeBottom + 16
		if want > joinY {
			joinY = want
		}
	}

	joinInput = rect{x: x, y: joinY, w: fieldW, h: fieldH}
	joinBtn = rect{x: x + fieldW + 10, y: joinY - 4, w: 120, h: btnH}
	return
}
