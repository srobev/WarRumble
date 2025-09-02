package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: splitgame <path/to/game.go>\n")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	inPath := flag.Arg(0)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inPath, nil, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	// Where to write
	outDir := filepath.Dir(inPath)
	pkgName := file.Name.Name

	// --- destination mapping ---
	funcMap := map[string]string{
		// app / lifecycle
		"New": "app.go", "Update": "app.go", "Draw": "app.go", "Layout": "app.go",
		"desktopMain": "app.go", "fitToScreen": "app.go",

		// login & simple helpers
		"updateLogin": "app.go", "logicalCursor": "app.go",

		// army helpers
		"rowsPerCol": "army.go", "trySaveArmy": "army.go", "buildArmyNames": "army.go",
		"validateArmy": "army.go", "selectedMinisList": "army.go",
		"setChampArmyFromSelected": "army.go", "loadSelectedForChampion": "army.go",
		"autoSaveCurrentChampionArmy": "army.go", "ensureArmyBgLayer": "army.go",
		"beginChampDrag": "army.go", "moveChampDrag": "army.go", "endChampDrag": "army.go",

		// battle
		"updateBattle": "battle.go", "drawBattleBar": "battle.go", "handRects": "battle.go",
		"hpfxStep": "battle.go", "drawHPBar": "battle.go",
		"safeName": "battle.go", "safeCost": "battle.go", "drawArenaBG": "map_render.go",

		// map / world
		"displayMapID": "map_render.go", "mapRenderRect": "map_render.go",
		"mapDstRect": "map_render.go", "mapRenderRectInBounds": "map_render.go",
		"mapEdgeColor": "map_render.go", "computeEdgeColorFromFS": "map_render.go",
		"ensureMapHotspots": "map_hotspots.go", "ensureMapRects": "map_hotspots.go",
		"arenaForHotspot": "map_hotspots.go",

		// PvP
		"pvpLayout": "pvp.go",

		// profile & avatars
		"listAvatars": "profile.go", "ensureAvatarImage": "profile.go",
		"drawProfileOverlay": "profile.go",

		// UI common
		"drawNineSlice": "ui_common.go", "drawNineBtn": "ui_common.go",

		// top/bottom bars
		"computeTopBarLayout": "topbar.go", "drawTopBarHome": "topbar.go",
		"computeBottomBarLayout": "bottombar.go", "drawBottomBar": "bottombar.go",

		// net / conn
		"retryConnect": "conn.go", "connectAsync": "conn.go",
		"ensureNet": "conn.go", "send": "conn.go", "resetToLogin": "conn.go",
		"resetToLoginNoAutoConnect": "conn.go", "requestLobbyDataOnce": "conn.go",

		// net handlers & actions
		"handle": "net_handlers.go", "onArmySave": "net_handlers.go",
		"onMapClicked": "net_handlers.go", "onStartBattle": "net_handlers.go",
		"onLeaveRoom": "net_handlers.go",

		// misc small helpers
		"trim": "app.go", "clampInt": "app.go", "mathMin": "app.go",
	}

	typeMap := map[string]string{
		// small types & enums into types.go
		"rect": "types.go", "hpFx": "battle.go", "NineSlice": "ui_common.go",
		"HitRect": "types.go", "Hotspot": "types.go", "HSRect": "types.go",
		"connState": "types.go", "screen": "types.go", "tab": "types.go",
		"Assets":      "assets.go",
		"battleHPBar": "battle.go",
	}

	constMap := map[string]string{
		// visual constants + screen enums into types.go
		"menuBarH": "types.go", "topBarH": "types.go", "battleHUDH": "types.go",
		"pad": "types.go", "btnW": "types.go", "btnH": "types.go", "rowH": "types.go",
		"defaultMapID": "types.go",
	}

	// per-file ASTs
	files := map[string]*ast.File{}
	mkfile := func(name string) *ast.File {
		if f := files[name]; f != nil {
			return f
		}
		f := &ast.File{Name: ast.NewIdent(pkgName)}
		// copy imports as-is; goimports will prune later
		for _, imp := range file.Imports {
			f.Imports = append(f.Imports, &ast.ImportSpec{Path: imp.Path, Name: imp.Name})
		}
		files[name] = f
		return f
	}

	appendDecl := func(dst string, d ast.Decl) {
		mkfile(dst).Decls = append(mkfile(dst).Decls, d)
	}

	// Walk declarations and distribute
	for _, d := range file.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			name := dd.Name.Name
			dst, ok := funcMap[name]
			if !ok {
				dst = "leftovers.go"
			}
			appendDecl(dst, dd)

		case *ast.GenDecl:
			switch dd.Tok {
			case token.TYPE:
				for _, sp := range dd.Specs {
					ts := sp.(*ast.TypeSpec)
					name := ts.Name.Name
					dst, ok := typeMap[name]
					if !ok {
						// Special-case Game: keep it in types.go so everyone sees it
						if name == "Game" {
							appendDecl("types.go", &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{ts}})
							continue
						}
						dst = "types.go"
					}
					appendDecl(dst, &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{ts}})
				}
			case token.CONST:
				// Split individual const specs so we can route by name
				for _, sp := range dd.Specs {
					vs := sp.(*ast.ValueSpec)
					// route each name; pack one Spec per GenDecl for simplicity
					for _, n := range vs.Names {
						name := n.Name
						dst, ok := constMap[name]
						if !ok {
							dst = "types.go"
						}
						ns := &ast.ValueSpec{Names: []*ast.Ident{ast.NewIdent(name)}, Type: vs.Type, Values: vs.Values, Doc: dd.Doc, Comment: vs.Comment}
						appendDecl(dst, &ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{ns}})
					}
				}
			case token.VAR:
				// keep vars together in leftovers unless they obviously belong elsewhere
				appendDecl("leftovers.go", dd)
			default:
				appendDecl("leftovers.go", dd)
			}
		default:
			appendDecl("leftovers.go", dd)
		}
	}

	// Ensure at least a banner comment in each new file
	for name, f := range files {
		if len(f.Decls) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString("// Code generated by splitgame; DO NOT EDIT.\n")
		b.WriteString("package ")
		b.WriteString(pkgName)
		b.WriteString("\n\n")

		// Pretty print AST
		var out strings.Builder
		if err := printer.Fprint(&out, fset, f); err != nil {
			panic(err)
		}
		src := []byte(b.String() + out.String())
		fmted, err := format.Source(src)
		if err == nil {
			src = fmted
		}
		if err := os.WriteFile(filepath.Join(outDir, name), src, 0644); err != nil {
			panic(err)
		}
		fmt.Println("wrote", name)
	}

	fmt.Println("— done. Now run: go run golang.org/x/tools/cmd/goimports@latest -w", outDir)
}
