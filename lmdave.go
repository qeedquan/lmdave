package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image/color"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/qeedquan/go-media/image/imageutil"
	"github.com/qeedquan/go-media/sdl"
	"github.com/qeedquan/go-media/sdl/sdlimage/sdlcolor"
)

const (
	TILE_SIZE     = 16
	DISPLAY_SCALE = 3
	MAX_SCORE     = 99999
)

type Assets struct {
	graphics_tiles [158]*sdl.Texture
}

type Level struct {
	path  [256]byte
	tiles [1000]byte
	_     [24]byte
}

type Monster struct {
	typ        int
	path_index int
	dead_timer int
	monster_x  int
	monster_y  int
	monster_px int
	monster_py int
	next_px    int
	next_py    int
}

type Game struct {
	saveslot        int
	pause           bool
	quit            bool
	tick            int
	dave_tick       int
	current_level   int
	lives           int
	score           int
	view_x          int
	view_y          int
	dave_x          int
	dave_y          int
	dave_dead_timer int
	dbullet_px      int
	dbullet_py      int
	dbullet_dir     int
	ebullet_px      int
	ebullet_py      int
	ebullet_dir     int
	dave_px         int
	dave_py         int
	on_ground       bool
	scroll_x        int
	last_dir        int

	dave_right      bool
	dave_left       bool
	dave_jump       bool
	dave_fire       bool
	dave_down       bool
	dave_up         bool
	dave_climb      bool
	dave_jetpack    bool
	jetpack_delay   int
	jump_timer      int
	try_right       bool
	try_left        bool
	try_jump        bool
	try_fire        bool
	try_jetpack     bool
	try_down        bool
	try_up          bool
	check_pickup_x  int
	check_pickup_y  int
	check_door      bool
	can_climb       bool
	collision_point [9]bool
	trophy          bool
	gun             bool
	jetpack         int

	monster [5]Monster

	level [10]Level
}

var (
	conf struct {
		res      string
		sav      string
		immortal bool
	}
	game     *Game
	assets   *Assets
	window   *sdl.Window
	renderer *sdl.Renderer
)

func main() {
	runtime.LockOSThread()
	log.SetPrefix("lmvdave: ")
	log.SetFlags(0)
	parse_flags()
	init_sdl()
	game = new_game()
	assets = new_assets()
	for !game.quit {
		timer_begin := sdl.GetTicks()

		game.check_input()
		game.update()
		game.render()

		timer_end := sdl.GetTicks()
		delay := 33 - (timer_end - timer_begin)
		if delay > 33 {
			delay = 0
		}
		sdl.Delay(delay)
	}
}

func parse_flags() {
	conf.res = filepath.Join(sdl.GetBasePath(), "res")
	conf.sav = filepath.Join(sdl.GetBasePath(), "sav")
	flag.StringVar(&conf.res, "res", conf.res, "resource directory")
	flag.StringVar(&conf.sav, "sav", conf.sav, "save directory")
	flag.BoolVar(&conf.immortal, "9lives", conf.immortal, "infinite lives")
	flag.Parse()
}

func init_sdl() {
	err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_TIMER)
	ck(err)

	w, h := 320, 200
	window, renderer, err = sdl.CreateWindowAndRenderer(w*DISPLAY_SCALE, h*DISPLAY_SCALE, sdl.WINDOW_RESIZABLE)
	ck(err)

	window.SetTitle("Dangerous Dave")
	window.SetMinimumSize(w, h)
	renderer.SetScale(DISPLAY_SCALE, DISPLAY_SCALE)
	renderer.SetLogicalSize(w, h)
	sdl.ShowCursor(0)
}

func ck(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func new_assets() *Assets {
	a := &Assets{}
	for i := 0; i < 158; i++ {
		fname := filepath.Join(conf.res, fmt.Sprintf("tile%d.png", i))
		img, err := imageutil.LoadRGBAFile(fname)
		ck(err)

		// Handle Dave tile masks
		if (i >= 53 && i <= 59) || i == 67 || i == 68 || (i >= 71 && i <= 73) || (i >= 77 && i <= 82) {
			var mask_offset int
			if i >= 53 && i <= 59 {
				mask_offset = 7
			} else if i >= 67 && i <= 68 {
				mask_offset = 2
			} else if i >= 71 && i <= 73 {
				mask_offset = 3
			} else if i >= 77 && i <= 82 {
				mask_offset = 6
			}

			mname := filepath.Join(conf.res, fmt.Sprintf("tile%d.png", i+mask_offset))
			mask, err := imageutil.LoadRGBAFile(mname)
			ck(err)

			r := mask.Bounds()
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					col := mask.At(x, y)
					if col != sdlcolor.Black {
						img.Set(x, y, color.RGBA{})
					}
				}
			}
		} else {
			// Monster tiles should use black transparency
			if (i >= 89 && i <= 120) || (i >= 129 && i <= 132) {
				img = imageutil.ColorKey(img, sdlcolor.Black)
			}
		}

		texture, err := renderer.CreateTextureFromImage(img)
		ck(err)
		a.graphics_tiles[i] = texture
	}
	return a
}

func new_game() *Game {
	c := &Game{}
	c.init()
	return c
}

func (c *Game) init() {
	c.saveslot = 0
	c.quit = false
	c.tick = 0
	c.current_level = 0
	c.lives = 3
	c.score = 0
	c.view_x = 0
	c.view_y = 0
	c.scroll_x = 0
	c.dave_x = 2
	c.dave_y = 8
	c.dbullet_px = 0
	c.dbullet_py = 0
	c.dbullet_dir = 0
	c.ebullet_px = 0
	c.ebullet_py = 0
	c.ebullet_dir = 0
	c.try_right = false
	c.try_left = false
	c.try_jump = false
	c.try_fire = false
	c.try_jetpack = false
	c.try_down = false
	c.dave_right = false
	c.dave_left = false
	c.dave_jump = false
	c.dave_fire = false
	c.dave_down = false
	c.dave_up = false
	c.dave_climb = false
	c.dave_jetpack = false
	c.jetpack_delay = 0
	c.last_dir = 0
	c.jump_timer = 0
	c.on_ground = true
	c.dave_px = c.dave_x * TILE_SIZE
	c.dave_py = c.dave_y * TILE_SIZE
	c.check_pickup_x = 0
	c.check_pickup_y = 0
	c.check_door = false

	for i := range c.monster {
		c.monster[i].typ = 0
	}
	c.load_levels()
}

func (c *Game) load_levels() {
	for i := 0; i < 10; i++ {
		name := filepath.Join(conf.res, fmt.Sprintf("level%d.dat", i))
		f, err := os.Open(name)
		ck(err)

		r := bufio.NewReader(f)
		err = binary.Read(r, binary.LittleEndian, &c.level[i].path)
		if err != nil {
			log.Fatalf("level%d: %v", i, err)
		}

		err = binary.Read(r, binary.LittleEndian, &c.level[i].tiles)
		if err != nil {
			log.Fatalf("level%d: %v", i, err)
		}

		f.Close()
	}
}

func (c *Game) check_input() {
	for {
		ev := sdl.PollEvent()
		if ev == nil {
			break
		}
		switch ev := ev.(type) {
		case sdl.QuitEvent:
			c.quit = true
		case sdl.KeyDownEvent:
			switch ev.Sym {
			case sdl.K_1:
				if c.saveslot > 0 {
					c.saveslot--
				}
				fmt.Println("Save Slot:", c.saveslot)
			case sdl.K_2:
				if c.saveslot < 10 {
					c.saveslot++
				}
				fmt.Println("Save Slot:", c.saveslot)
			case sdl.K_F2:
				c.save_state()
			case sdl.K_F4:
				c.load_state()
			case sdl.K_BACKSPACE:
				c.pause = !c.pause
				fmt.Println("Paused:", c.pause)
			}
		}
	}
	keystate := sdl.GetKeyboardState()
	if keystate[sdl.SCANCODE_RIGHT] != 0 {
		c.try_right = true
	}
	if keystate[sdl.SCANCODE_LEFT] != 0 {
		c.try_left = true
	}
	if keystate[sdl.SCANCODE_UP] != 0 {
		c.try_jump = true
	}
	if keystate[sdl.SCANCODE_DOWN] != 0 {
		c.try_down = true
	}
	if keystate[sdl.SCANCODE_A] != 0 {
		c.try_fire = true
	}
	if keystate[sdl.SCANCODE_B] != 0 {
		c.try_jetpack = true
	}
	if keystate[sdl.SCANCODE_ESCAPE] != 0 {
		c.quit = true
	}
}

func (c *Game) update() {
	if c.pause {
		c.clear_input()
		return
	}
	c.check_collision()
	c.pickup_item(c.check_pickup_x, c.check_pickup_y)
	c.update_dbullet()
	c.update_ebullet()
	c.verify_input()
	c.move_dave()
	c.move_monsters()
	c.fire_monsters()
	c.scroll_screen()
	c.apply_gravity()
	c.update_level()
	c.clear_input()
}

// Renders the world. First step of the game loop
func (c *Game) render() {
	renderer.SetDrawColor(sdlcolor.Black)
	renderer.Clear()
	c.draw_world()
	c.draw_dave()
	c.draw_dave_bullet()
	c.draw_monster_bullet()
	c.draw_monsters()
	c.draw_ui()
	renderer.Present()
}

// Updates Dave's collision point state
func (c *Game) check_collision() {
	// Updates 8 points around Dave
	c.collision_point[0] = c.is_clear(c.dave_px+4, c.dave_py-1, true)
	c.collision_point[1] = c.is_clear(c.dave_px+10, c.dave_py-1, true)
	c.collision_point[2] = c.is_clear(c.dave_px+11, c.dave_py+4, true)
	c.collision_point[3] = c.is_clear(c.dave_px+11, c.dave_py+12, true)
	c.collision_point[4] = c.is_clear(c.dave_px+10, c.dave_py+16, true)
	c.collision_point[5] = c.is_clear(c.dave_px+4, c.dave_py+16, true)
	c.collision_point[6] = c.is_clear(c.dave_px+3, c.dave_py+12, true)
	c.collision_point[7] = c.is_clear(c.dave_px+3, c.dave_py+4, true)

	/* Is dave on the ground? */
	c.on_ground = ((!c.collision_point[4] && !c.collision_point[5]) || c.dave_climb)

	grid_x := (c.dave_px + 6) / TILE_SIZE
	grid_y := (c.dave_py + 8) / TILE_SIZE

	/* Don't check outside the room */
	typ := uint8(0)
	if grid_x < 100 && grid_y < 10 {
		typ = c.level[c.current_level].tiles[grid_y*100+grid_x]
	}

	if (typ >= 33 && typ <= 35) || typ == 41 {
		c.can_climb = true
	} else {
		c.can_climb = false
		c.dave_climb = false
	}
}

func (c *Game) clear_input() {
	c.try_jump = false
	c.try_right = false
	c.try_left = false
	c.try_fire = false
	c.try_jetpack = false
	c.try_down = false
	c.try_up = false
}

// Pickup item at input location
func (c *Game) pickup_item(grid_x, grid_y int) {
	// No pickups outside of the world (or you'll be eaten by a grue)
	if grid_x == 0 || grid_y == 0 || grid_x > 100 || grid_y > 10 {
		return
	}

	// Get the type
	typ := c.level[c.current_level].tiles[grid_y*100+grid_x]

	// Handle the type
	switch typ {
	// Jetpack pickup
	case 4:
		c.jetpack = 0xff
	// Trophy pickup
	case 10:
		c.add_score(1000)
		c.trophy = true
	// Gun pickup
	case 20:
		c.gun = true
	// Collectibles pickup
	case 47:
		c.add_score(100)
	case 48:
		c.add_score(50)
	case 49:
		c.add_score(150)
	case 50:
		c.add_score(300)
	case 51:
		c.add_score(200)
	case 52:
		c.add_score(500)
	}

	// Clear the pickup tile
	c.level[c.current_level].tiles[grid_y*100+grid_x] = 0

	// Clear the pickup handler
	c.check_pickup_x = 0
	c.check_pickup_y = 0
}

// Move Dave's bullets
func (c *Game) update_dbullet() {
	grid_x := c.dbullet_px / TILE_SIZE
	grid_y := c.dbullet_py / TILE_SIZE

	// Not active
	if c.dbullet_px == 0 || c.dbullet_py == 0 {
		return
	}

	// Bullet hit something - deactivate
	if !c.is_clear(c.dbullet_px, c.dbullet_py, false) {
		c.dbullet_px, c.dbullet_py = 0, 0
	}

	// Bullet left room - deactivate
	if grid_x-c.view_x < 1 || grid_x-c.view_x > 20 {
		c.dbullet_px, c.dbullet_py = 0, 0
	}

	if c.dbullet_px != 0 {
		c.dbullet_px += c.dbullet_dir * 4

		/* Check all monster positions */
		for i := range c.monster {
			if c.monster[i].typ != 0 {
				mx := c.monster[i].monster_x
				my := c.monster[i].monster_y
				if (grid_y == my || grid_y == my+1) && (grid_x == mx || grid_x == mx+1) {
					// Dave's bullet hits monster
					c.dbullet_px, c.dbullet_py = 0, 0
					c.monster[i].dead_timer = 30
					c.add_score(300)
				}
			}
		}
	}
}

// Move enemies bullets
func (c *Game) update_ebullet() {
	if c.ebullet_px == 0 || c.ebullet_py == 0 {
		return
	}

	if !c.is_clear(c.ebullet_px, c.ebullet_py, false) {
		c.ebullet_px, c.ebullet_py = 0, 0
	}

	if !c.is_visible(c.ebullet_px) {
		c.ebullet_px, c.ebullet_py = 0, 0
	}

	if c.ebullet_px != 0 {
		c.ebullet_px += c.ebullet_dir * 4

		grid_x := c.ebullet_px / TILE_SIZE
		grid_y := c.ebullet_py / TILE_SIZE

		/* Compare with Dave's position */
		if grid_y == c.dave_y && grid_x == c.dave_x {
			// Monster's bullet hits Dave
			c.ebullet_px, c.ebullet_py = 0, 0
			c.dave_dead_timer = 30
		}
	}
}

// Starts a new level
func (c *Game) start_level() {
	c.restart_level()

	// Deactivate monsters
	for i := range c.monster {
		c.monster[i].typ = 0
		c.monster[i].path_index = 0
		c.monster[i].dead_timer = 0
		c.monster[i].next_px = 0
		c.monster[i].next_py = 0
	}

	// Activate monsters based on level
	// Current level starts at 0
	switch c.current_level {
	case 2:
		c.monster[0].typ = 89
		c.monster[0].monster_px = 44 * TILE_SIZE
		c.monster[0].monster_py = 4 * TILE_SIZE
		c.monster[1].typ = 89
		c.monster[1].monster_px = 59 * TILE_SIZE
		c.monster[1].monster_py = 4 * TILE_SIZE
	case 3:
		c.monster[0].typ = 93
		c.monster[0].monster_px = 32 * TILE_SIZE
		c.monster[0].monster_py = 2 * TILE_SIZE
	case 4:
		c.monster[0].typ = 97
		c.monster[0].monster_px = 15 * TILE_SIZE
		c.monster[0].monster_py = 3 * TILE_SIZE
	case 5:
		c.monster[0].typ = 101
		c.monster[0].monster_px = 10 * TILE_SIZE
		c.monster[0].monster_py = 8 * TILE_SIZE
		c.monster[1].typ = 101
		c.monster[1].monster_px = 28 * TILE_SIZE
		c.monster[1].monster_py = 8 * TILE_SIZE
		c.monster[2].typ = 101
		c.monster[2].monster_px = 45 * TILE_SIZE
		c.monster[2].monster_py = 2 * TILE_SIZE
		c.monster[3].typ = 101
		c.monster[3].monster_px = 40 * TILE_SIZE
		c.monster[3].monster_py = 8 * TILE_SIZE
	case 6:
		c.monster[0].typ = 105
		c.monster[0].monster_px = 5 * TILE_SIZE
		c.monster[0].monster_py = 2 * TILE_SIZE
		c.monster[1].typ = 105
		c.monster[1].monster_px = 16 * TILE_SIZE
		c.monster[1].monster_py = 1 * TILE_SIZE
		c.monster[2].typ = 105
		c.monster[2].monster_px = 46 * TILE_SIZE
		c.monster[2].monster_py = 2 * TILE_SIZE
		c.monster[3].typ = 105
		c.monster[3].monster_px = 56 * TILE_SIZE
		c.monster[3].monster_py = 3 * TILE_SIZE
	case 7:
		c.monster[0].typ = 109
		c.monster[0].monster_px = 53 * TILE_SIZE
		c.monster[0].monster_py = 5 * TILE_SIZE
		c.monster[1].typ = 109
		c.monster[1].monster_px = 72 * TILE_SIZE
		c.monster[1].monster_py = 2 * TILE_SIZE
		c.monster[2].typ = 109
		c.monster[2].monster_px = 84 * TILE_SIZE
		c.monster[2].monster_py = 1 * TILE_SIZE
	case 8:
		c.monster[0].typ = 113
		c.monster[0].monster_px = 35 * TILE_SIZE
		c.monster[0].monster_py = 8 * TILE_SIZE
		c.monster[1].typ = 113
		c.monster[1].monster_px = 41 * TILE_SIZE
		c.monster[1].monster_py = 8 * TILE_SIZE
		c.monster[2].typ = 113
		c.monster[2].monster_px = 49 * TILE_SIZE
		c.monster[2].monster_py = 8 * TILE_SIZE
		c.monster[3].typ = 113
		c.monster[3].monster_px = 65 * TILE_SIZE
		c.monster[3].monster_py = 8 * TILE_SIZE
	case 9:
		c.monster[0].typ = 117
		c.monster[0].monster_px = 45 * TILE_SIZE
		c.monster[0].monster_py = 8 * TILE_SIZE
		c.monster[1].typ = 117
		c.monster[1].monster_px = 51 * TILE_SIZE
		c.monster[1].monster_py = 2 * TILE_SIZE
		c.monster[2].typ = 117
		c.monster[2].monster_px = 65 * TILE_SIZE
		c.monster[2].monster_py = 3 * TILE_SIZE
		c.monster[3].typ = 117
		c.monster[3].monster_px = 82 * TILE_SIZE
		c.monster[3].monster_py = 5 * TILE_SIZE
	}

	// Reset various state variables at start of each level
	c.dave_dead_timer = 0
	c.trophy = false
	c.gun = false
	c.jetpack = 0
	c.dave_jetpack = false
	c.check_door = false
	c.view_x = 0
	c.view_y = 0
	c.last_dir = 0
	c.dbullet_px = 0
	c.dbullet_py = 0
	c.ebullet_px = 0
	c.ebullet_py = 0
	c.jump_timer = 0
}

// Check if keyboard input is valid. If so, set action variable
func (c *Game) verify_input() {
	// Dave is dead. No input is valid
	if c.dave_dead_timer != 0 {
		return
	}

	// Dave can move right if there are no obstructions
	if c.try_right && c.collision_point[2] && c.collision_point[3] {
		c.dave_right = true
	}

	// Dave can move left if there are no obstructions
	if c.try_left && c.collision_point[6] && c.collision_point[7] {
		c.dave_left = true
	}

	// Dave can jumpif he's on the ground and not using the jetpack
	if c.try_jump && c.on_ground && !c.dave_jump && !c.dave_jetpack && !c.can_climb && c.collision_point[0] && c.collision_point[1] {
		c.dave_jump = true
	}

	// Dave should climb rather than jump if he's in front of a climable tile
	if c.try_jump && c.can_climb {
		c.dave_up = true
		c.dave_climb = true
	}

	// Dave can fire if he has the gun and isn't already firing
	if c.try_fire && c.gun && c.dbullet_py == 0 && c.dbullet_px == 0 {
		c.dave_fire = true
	}

	// Dave can toggle the jetpack if he has one and he didn't recently toggle it
	if c.try_jetpack && c.jetpack != 0 && c.jetpack_delay == 0 {
		c.dave_jetpack = !c.dave_jetpack
		c.jetpack_delay = 10
	}

	// Dave can move downward if he is climbing or has a jetpack
	if c.try_down && (c.dave_jetpack || c.dave_climb) && c.collision_point[4] && c.collision_point[5] {
		c.dave_down = true
	}

	// Dave can move up if he has a jetpack
	if c.try_jump && c.dave_jetpack && c.collision_point[0] && c.collision_point[1] {
		c.dave_up = true
	}
}

// Move Dave around the world
func (c *Game) move_dave() {
	c.dave_x = c.dave_px / TILE_SIZE
	c.dave_y = c.dave_py / TILE_SIZE

	// Wrap Dave to the top of the level when he falls through the floor
	if c.dave_y > 9 {
		c.dave_y = 0
		c.dave_py = -16
	}

	// Move Dave right
	if c.dave_right {
		c.dave_px += 2
		c.last_dir = 1
		c.dave_tick++
		c.dave_right = false
	}

	// Move Dave left
	if c.dave_left {
		c.dave_px -= 2
		c.last_dir = -1
		c.dave_tick++
		c.dave_left = false
	}

	// Move Dave down
	if c.dave_down {
		c.dave_tick++
		c.dave_py += 2
		c.dave_down = false
	}

	// Move Dave up
	if c.dave_up {
		c.dave_tick++
		c.dave_py -= 2
		c.dave_up = false
	}

	// Jetpack usage cancels jump effects
	if c.dave_jetpack {
		c.dave_jump = false
		c.jump_timer = 0
	}

	// Make Dave jump
	if c.dave_jump {
		if c.jump_timer == 0 {
			c.jump_timer = 30
			c.last_dir = 0
		}

		if c.collision_point[0] && c.collision_point[1] {
			// Dave should move up at a decreasing rate, then float for a moment
			if c.jump_timer > 16 {
				c.dave_py -= 2
			}
			if c.jump_timer >= 12 && c.jump_timer <= 15 {
				c.dave_py -= 1
			}
		}

		if c.jump_timer--; c.jump_timer == 0 {
			c.dave_jump = false
		}
	}

	// Fire Dave's gun
	if c.dave_fire {
		c.dbullet_dir = c.last_dir

		// Bullet should match Dave's direction
		if c.dbullet_dir == 0 {
			c.dbullet_dir = 1
		}

		// Bullet should start in front of Dave
		if c.dbullet_dir == 1 {
			c.dbullet_px = c.dave_px + 18
		}

		if c.dbullet_dir == -1 {
			c.dbullet_px = c.dave_py - 8
		}

		c.dbullet_py = c.dave_py + 8
		c.dave_fire = false
	}
}

// Move monsters along their predefined path
func (c *Game) move_monsters() {
	for i := range c.monster {
		m := &c.monster[i]
		if m.typ != 0 && m.dead_timer == 0 {
			// Move monster twice each tick. Hack to match speed of original game
			for j := 0; j < 2; j++ {
				if m.next_px == 0 && m.next_py == 0 {
					// Get the next path waypoint
					m.next_px = int(c.level[c.current_level].path[m.path_index])
					m.next_py = int(c.level[c.current_level].path[m.path_index+1])
					m.path_index += 2

					// End of path -- reset monster to start of path
					if m.next_px == -22 && m.next_py == -22 {
						m.next_px = int(c.level[c.current_level].path[0])
						m.next_py = int(c.level[c.current_level].path[1])
						m.path_index = 2
					}
				}

				// Move monster left
				if m.next_px < 0 {
					m.monster_px -= 1
					m.next_px++
				}

				// Move monster right
				if m.next_px > 0 {
					m.monster_px += 1
					m.next_px--
				}

				// Move monster up
				if m.next_py < 0 {
					m.monster_py -= 1
					m.next_py++
				}

				// Move monster down
				if m.next_py > 0 {
					m.monster_py += 1
					m.next_py--
				}
			}

			// Update monster grid position
			m.monster_x = m.monster_px / TILE_SIZE
			m.monster_y = m.monster_py / TILE_SIZE
		}
	}
}

// Monster shooting
func (c *Game) fire_monsters() {
	if c.ebullet_px == 0 && c.ebullet_py == 0 {
		for i := range c.monster {
			// Monster's shoot if they're active and visible
			if c.monster[i].typ != 0 && c.is_visible(c.monster[i].monster_px) && c.monster[i].dead_timer == 0 {
				// Shoot towards Dave
				if c.dave_px < c.monster[i].monster_px {
					c.ebullet_dir = -1
				} else {
					c.ebullet_dir = 1
				}

				if c.ebullet_dir == 0 {
					c.ebullet_dir = 1
				}

				// Start bullet in front of monster
				if c.ebullet_dir == 1 {
					c.ebullet_px = c.monster[i].monster_px + 18
				} else if c.ebullet_dir == -1 {
					c.ebullet_px = c.monster[i].monster_px - 8
				}

				c.ebullet_py = c.monster[i].monster_py + 8
			}
		}
	}
}

// Scroll the screen when Dave is near the edge
// Game view is 20 grid units wide
func (c *Game) scroll_screen() {
	// Scroll right if Dave reaches view position 18
	if c.dave_x-c.view_x >= 18 {
		c.scroll_x = 15
	}

	// Scroll left if Dave reaches position 2
	if c.dave_x-c.view_x < 2 {
		c.scroll_x = -15
	}

	if c.scroll_x > 0 {
		// Cap right side at 80 (each level is 100 wide)
		if c.view_x == 80 {
			c.scroll_x = 0
		} else {
			c.view_x++
			c.scroll_x--
		}
	}

	// Cap left side at 0
	if c.scroll_x < 0 {
		if c.view_x == 0 {
			c.scroll_x = 0
		} else {
			c.view_x--
			c.scroll_x++
		}
	}
}

// Apply gravity to Dave
func (c *Game) apply_gravity() {
	if !c.dave_jump && !c.on_ground && !c.dave_jetpack && !c.dave_climb {
		if c.is_clear(c.dave_px+4, c.dave_py+17, true) {
			c.dave_py += 2
		} else {
			not_align := c.dave_py % TILE_SIZE

			// If Dave is not level aligned, lock him to nearest tile
			if not_align != 0 {
				if not_align < 8 {
					c.dave_py -= not_align
				} else {
					c.dave_py += TILE_SIZE - not_align
				}
			}
		}
	}
}

// Handle level-wide events
func (c *Game) update_level() {
	// Increment game tick timer
	c.tick++

	// Decrement jetpack delay
	if c.jetpack_delay != 0 {
		c.jetpack_delay--
	}

	// Decrement Dave's jetpack fuel
	if c.dave_jetpack {
		if c.jetpack--; c.jetpack == 0 {
			c.dave_jetpack = false
		}
	}

	// Check if Dave completes level
	if c.check_door {
		if c.trophy {
			c.add_score(2000)
			if c.current_level < 9 {
				c.current_level++
				c.start_level()
			} else {
				fmt.Printf("You won with %d points\n", c.score)
				c.quit = true
			}
		} else {
			c.check_door = false
		}
	}

	// Reset level when Dave is dead
	if c.dave_dead_timer != 0 {
		if c.dave_dead_timer--; c.dave_dead_timer == 0 {
			if c.lives != 0 {
				if !conf.immortal {
					c.lives--
				}
				c.restart_level()
			} else {
				c.quit = true
			}
		}
	}

	// Check monster timers
	for i := range c.monster {
		if c.monster[i].dead_timer != 0 {
			if c.monster[i].dead_timer--; c.monster[i].dead_timer == 0 {
				c.monster[i].typ = 0
			}
		}
	}
}

// Sets Dave start position in a level
func (c *Game) restart_level() {
	switch c.current_level {
	case 0:
		c.dave_x, c.dave_y = 2, 8
	case 1:
		c.dave_x, c.dave_y = 1, 8
	case 2:
		c.dave_x, c.dave_y = 2, 5
	case 3:
		c.dave_x, c.dave_y = 1, 5
	case 4:
		c.dave_x, c.dave_y = 2, 8
	case 5:
		c.dave_x, c.dave_y = 2, 8
	case 6:
		c.dave_x, c.dave_y = 1, 2
	case 7:
		c.dave_x, c.dave_y = 2, 8
	case 8:
		c.dave_x, c.dave_y = 6, 1
	case 9:
		c.dave_x, c.dave_y = 2, 8
	}
	c.dave_px = c.dave_x * TILE_SIZE
	c.dave_py = c.dave_y * TILE_SIZE
}

// Update frame animation based on tick timer and tile's type
func (c *Game) update_frame(tile, salt int) int {
	var mod int
	switch tile {
	case 6:
		mod = 4
	case 10:
		mod = 5
	case 25:
		mod = 4
	case 36:
		mod = 5
	case 129:
		mod = 4
	default:
		mod = 1
	}
	return tile + (salt+c.tick/5)%mod
}

// Render the world
func (c *Game) draw_world() {
	// Draw each tile in row-major
	var dest sdl.Rect
	for j := 0; j < 10; j++ {
		dest.Y = TILE_SIZE + int32(j)*TILE_SIZE
		dest.W = TILE_SIZE
		dest.H = TILE_SIZE
		for i := 0; i < 20; i++ {
			dest.X = int32(i) * TILE_SIZE
			tile_index := int(c.level[c.current_level].tiles[j*100+c.view_x+i])
			tile_index = c.update_frame(tile_index, i)
			renderer.Copy(assets.graphics_tiles[tile_index], nil, &dest)
		}
	}
}

func (c *Game) draw_dave() {
	var dest sdl.Rect
	dest.X = int32(c.dave_px - c.view_x*TILE_SIZE)
	dest.Y = int32(TILE_SIZE + c.dave_py)
	dest.W = 20
	dest.H = 16

	// Find the right Dave tile based on his condition
	var tile_index int
	if c.last_dir == 0 {
		tile_index = 56
	} else {
		tile_index = 57
		if c.last_dir > 0 {
			tile_index = 53
		}
		tile_index += (c.dave_tick / 5) % 3
	}

	if c.dave_jetpack {
		tile_index = 80
		if c.last_dir >= 10 {
			tile_index = 77
		}
	} else {
		if c.dave_jump || !c.on_ground {
			tile_index = 68
			if c.last_dir >= 0 {
				tile_index = 67
			}
		}
		if c.dave_climb {
			tile_index = 71 + (c.dave_tick/5)%3
		}
	}

	if c.dave_dead_timer != 0 {
		tile_index = 129 + (c.tick/3)%4
	}

	renderer.Copy(assets.graphics_tiles[tile_index], nil, &dest)
}

func (c *Game) draw_dave_bullet() {
	if c.dbullet_px == 0 || c.dbullet_py == 0 {
		return
	}

	var dest sdl.Rect
	dest.X = int32(c.dbullet_px - c.view_x*TILE_SIZE)
	dest.Y = int32(TILE_SIZE + c.dbullet_py)
	dest.W = 12
	dest.H = 3

	tile_index := 128
	if c.dbullet_dir > 0 {
		tile_index = 127
	}
	renderer.Copy(assets.graphics_tiles[tile_index], nil, &dest)
}

func (c *Game) draw_monster_bullet() {
	if c.ebullet_px != 0 {
		var dest sdl.Rect
		dest.X = int32(c.ebullet_px - c.view_x*TILE_SIZE)
		dest.Y = int32(TILE_SIZE + c.ebullet_py)
		dest.W = 12
		dest.H = 3
		tile_index := 124
		if c.ebullet_dir > 0 {
			tile_index = 121
		}

		renderer.Copy(assets.graphics_tiles[tile_index], nil, &dest)
	}
}

func (c *Game) draw_monsters() {
	for i := range c.monster {
		m := &c.monster[i]
		if m.typ != 0 {
			var dest sdl.Rect
			dest.X = int32(m.monster_px - c.view_x*TILE_SIZE)
			dest.Y = int32(TILE_SIZE + m.monster_py)
			dest.W = 20
			dest.H = 16

			tile_index := m.typ
			if m.dead_timer != 0 {
				tile_index = 129
			}
			tile_index += (c.tick / 3) % 4

			if m.typ >= 105 && m.typ <= 108 && m.dead_timer == 0 {
				dest.W = 18
				dest.H = 8
			}

			renderer.Copy(assets.graphics_tiles[tile_index], nil, &dest)
		}
	}
}

func (c *Game) draw_ui() {
	// Screen border
	dest := sdl.Rect{0, 16, 960, 1}
	renderer.SetDrawColor(sdl.Color{0xEE, 0xEE, 0xEE, 0xFF})
	renderer.FillRect(&dest)
	dest.Y = 176
	renderer.FillRect(&dest)

	// Score banner
	dest = sdl.Rect{1, 2, 62, 11}
	renderer.Copy(assets.graphics_tiles[137], nil, &dest)

	// Level banner
	dest.X = 120
	renderer.Copy(assets.graphics_tiles[136], nil, &dest)

	// Lives banner
	dest.X = 200
	renderer.Copy(assets.graphics_tiles[135], nil, &dest)

	// Score 10000s digit
	dest = sdl.Rect{64, 2, 8, 11}
	renderer.Copy(assets.graphics_tiles[148+(c.score/10000)%10], nil, &dest)

	// Score 1000s digit
	dest.X = 72
	renderer.Copy(assets.graphics_tiles[148+(c.score/1000)%10], nil, &dest)

	// Score 100s digit
	dest.X = 80
	renderer.Copy(assets.graphics_tiles[148+(c.score/100)%10], nil, &dest)

	// Score 10s digit
	dest.X = 88
	renderer.Copy(assets.graphics_tiles[148+(c.score/10)%10], nil, &dest)

	// Score LSD is always zero
	dest.X = 96
	renderer.Copy(assets.graphics_tiles[148], nil, &dest)

	// Level 10s digit
	dest.X = 170
	renderer.Copy(assets.graphics_tiles[148+(c.current_level+1)/10], nil, &dest)

	// Level unit digit
	dest.X = 178
	renderer.Copy(assets.graphics_tiles[148+(c.current_level+1)/10], nil, &dest)

	// Life count icon
	for i := 0; i < c.lives; i++ {
		dest.X = int32(255 + 16*i)
		dest.W = 16
		dest.H = 12
		renderer.Copy(assets.graphics_tiles[143], nil, &dest)
	}

	// Trophy pickup banner
	if c.trophy {
		dest = sdl.Rect{72, 180, 176, 14}
		renderer.Copy(assets.graphics_tiles[138], nil, &dest)
	}

	// Gun pickup banner
	if c.gun {
		dest = sdl.Rect{255, 180, 62, 11}
		renderer.Copy(assets.graphics_tiles[134], nil, &dest)
	}

	// Jetpack UI elements
	if c.jetpack != 0 {
		// Jetpack banner
		dest = sdl.Rect{1, 177, 62, 11}
		renderer.Copy(assets.graphics_tiles[133], nil, &dest)

		// Jetpack fuel counter
		dest = sdl.Rect{1, 190, 62, 8}
		renderer.Copy(assets.graphics_tiles[141], nil, &dest)

		// Jetpack fuel bar
		dest = sdl.Rect{2, 192, int32(float64(c.jetpack) * 0.23), 4}
		renderer.SetDrawColor(sdl.Color{0xEE, 0x00, 0x00, 0xFF})
		renderer.FillRect(&dest)
	}
}

// Checks if designated grid has an obstruction or pickup, true means clear
func (c *Game) is_clear(px, py int, is_dave bool) bool {
	grid_x := px / TILE_SIZE
	grid_y := py / TILE_SIZE
	if grid_x < 0 || grid_x > 99 || grid_y < 0 || grid_y > 9 {
		return true
	}
	typ := c.level[c.current_level].tiles[grid_y*100+grid_x]
	switch typ {
	case 1, 3, 5, 15, 16, 17, 18, 19, 21, 22, 23, 24, 29, 30:
		return false
	}

	// Dave-only collision checks (pickups)
	if is_dave {
		switch typ {
		case 2:
			c.check_door = true
		case 4, 10, 20, 47, 48, 49, 50, 51, 52:
			c.check_pickup_x = grid_x
			c.check_pickup_y = grid_y
		case 6, 25, 36:
			if c.dave_dead_timer == 0 {
				c.dave_dead_timer = 30
			}
		}
	}

	return true
}

// Checks if an input pixel position is currently visible
func (c *Game) is_visible(px int) bool {
	pos_x := px / TILE_SIZE
	return pos_x-c.view_x < 20 && pos_x-c.view_x >= 0
}

// Adds to player score and checks for extra life every 20,000 points
func (c *Game) add_score(new_score int) {
	if c.score/20000 != (c.score+new_score)/20000 {
		c.lives++
	}
	c.score += new_score
	if c.score >= MAX_SCORE {
		c.score = MAX_SCORE
	}
}

func (c *Game) save_state() {
	b := new(bytes.Buffer)
	fmt.Fprintf(b, "%v\n", c.tick)
	fmt.Fprintf(b, "%v\n", c.dave_tick)
	fmt.Fprintf(b, "%v\n", c.current_level)
	fmt.Fprintf(b, "%v\n", c.lives)
	fmt.Fprintf(b, "%v\n", c.score)
	fmt.Fprintf(b, "%v\n", c.view_x)
	fmt.Fprintf(b, "%v\n", c.view_y)
	fmt.Fprintf(b, "%v\n", c.dave_x)
	fmt.Fprintf(b, "%v\n", c.dave_y)
	fmt.Fprintf(b, "%v\n", c.dave_dead_timer)
	fmt.Fprintf(b, "%v\n", c.dbullet_px)
	fmt.Fprintf(b, "%v\n", c.dbullet_py)
	fmt.Fprintf(b, "%v\n", c.dbullet_dir)
	fmt.Fprintf(b, "%v\n", c.ebullet_px)
	fmt.Fprintf(b, "%v\n", c.ebullet_py)
	fmt.Fprintf(b, "%v\n", c.ebullet_dir)
	fmt.Fprintf(b, "%v\n", c.dave_px)
	fmt.Fprintf(b, "%v\n", c.dave_py)
	fmt.Fprintf(b, "%v\n", c.on_ground)
	fmt.Fprintf(b, "%v\n", c.scroll_x)
	fmt.Fprintf(b, "%v\n", c.last_dir)
	fmt.Fprintf(b, "%v\n", c.dave_right)
	fmt.Fprintf(b, "%v\n", c.dave_left)
	fmt.Fprintf(b, "%v\n", c.dave_jump)
	fmt.Fprintf(b, "%v\n", c.dave_fire)
	fmt.Fprintf(b, "%v\n", c.dave_down)
	fmt.Fprintf(b, "%v\n", c.dave_up)
	fmt.Fprintf(b, "%v\n", c.dave_climb)
	fmt.Fprintf(b, "%v\n", c.dave_jetpack)
	fmt.Fprintf(b, "%v\n", c.jetpack_delay)
	fmt.Fprintf(b, "%v\n", c.jump_timer)
	fmt.Fprintf(b, "%v\n", c.try_right)
	fmt.Fprintf(b, "%v\n", c.try_left)
	fmt.Fprintf(b, "%v\n", c.try_jump)
	fmt.Fprintf(b, "%v\n", c.try_fire)
	fmt.Fprintf(b, "%v\n", c.try_jetpack)
	fmt.Fprintf(b, "%v\n", c.try_down)
	fmt.Fprintf(b, "%v\n", c.try_up)
	fmt.Fprintf(b, "%v\n", c.check_pickup_x)
	fmt.Fprintf(b, "%v\n", c.check_pickup_y)
	fmt.Fprintf(b, "%v\n", c.check_door)
	fmt.Fprintf(b, "%v\n", c.can_climb)
	fmt.Fprintf(b, "%v\n", c.try_left)
	for i := range c.collision_point {
		fmt.Fprintf(b, "%v\n", c.collision_point[i])
	}
	fmt.Fprintf(b, "%v\n", c.trophy)
	fmt.Fprintf(b, "%v\n", c.gun)
	fmt.Fprintf(b, "%v\n", c.jetpack)
	for _, m := range c.monster {
		fmt.Fprintf(b, "%v\n", m.typ)
		fmt.Fprintf(b, "%v\n", m.path_index)
		fmt.Fprintf(b, "%v\n", m.dead_timer)
		fmt.Fprintf(b, "%v\n", m.monster_x)
		fmt.Fprintf(b, "%v\n", m.monster_y)
		fmt.Fprintf(b, "%v\n", m.monster_px)
		fmt.Fprintf(b, "%v\n", m.monster_py)
		fmt.Fprintf(b, "%v\n", m.next_px)
		fmt.Fprintf(b, "%v\n", m.next_py)
	}

	os.MkdirAll(conf.sav, 0755)
	name := filepath.Join(conf.sav, fmt.Sprintf("%d.sav", c.saveslot))
	err := ioutil.WriteFile(name, b.Bytes(), 0644)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Saved state to slot", c.saveslot)
	}
}

func (c *Game) load_state() {
	name := filepath.Join(conf.sav, fmt.Sprintf("%d.sav", c.saveslot))
	f, err := os.Open(name)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	b := bufio.NewReader(f)
	fmt.Fscanf(b, "%v\n", &c.tick)
	fmt.Fscanf(b, "%v\n", &c.dave_tick)
	fmt.Fscanf(b, "%v\n", &c.current_level)
	fmt.Fscanf(b, "%v\n", &c.lives)
	fmt.Fscanf(b, "%v\n", &c.score)
	fmt.Fscanf(b, "%v\n", &c.view_x)
	fmt.Fscanf(b, "%v\n", &c.view_y)
	fmt.Fscanf(b, "%v\n", &c.dave_x)
	fmt.Fscanf(b, "%v\n", &c.dave_y)
	fmt.Fscanf(b, "%v\n", &c.dave_dead_timer)
	fmt.Fscanf(b, "%v\n", &c.dbullet_px)
	fmt.Fscanf(b, "%v\n", &c.dbullet_py)
	fmt.Fscanf(b, "%v\n", &c.dbullet_dir)
	fmt.Fscanf(b, "%v\n", &c.ebullet_px)
	fmt.Fscanf(b, "%v\n", &c.ebullet_py)
	fmt.Fscanf(b, "%v\n", &c.ebullet_dir)
	fmt.Fscanf(b, "%v\n", &c.dave_px)
	fmt.Fscanf(b, "%v\n", &c.dave_py)
	fmt.Fscanf(b, "%v\n", &c.on_ground)
	fmt.Fscanf(b, "%v\n", &c.scroll_x)
	fmt.Fscanf(b, "%v\n", &c.last_dir)
	fmt.Fscanf(b, "%v\n", &c.dave_right)
	fmt.Fscanf(b, "%v\n", &c.dave_left)
	fmt.Fscanf(b, "%v\n", &c.dave_jump)
	fmt.Fscanf(b, "%v\n", &c.dave_fire)
	fmt.Fscanf(b, "%v\n", &c.dave_down)
	fmt.Fscanf(b, "%v\n", &c.dave_up)
	fmt.Fscanf(b, "%v\n", &c.dave_climb)
	fmt.Fscanf(b, "%v\n", &c.dave_jetpack)
	fmt.Fscanf(b, "%v\n", &c.jetpack_delay)
	fmt.Fscanf(b, "%v\n", &c.jump_timer)
	fmt.Fscanf(b, "%v\n", &c.try_right)
	fmt.Fscanf(b, "%v\n", &c.try_left)
	fmt.Fscanf(b, "%v\n", &c.try_jump)
	fmt.Fscanf(b, "%v\n", &c.try_fire)
	fmt.Fscanf(b, "%v\n", &c.try_jetpack)
	fmt.Fscanf(b, "%v\n", &c.try_down)
	fmt.Fscanf(b, "%v\n", &c.try_up)
	fmt.Fscanf(b, "%v\n", &c.check_pickup_x)
	fmt.Fscanf(b, "%v\n", &c.check_pickup_y)
	fmt.Fscanf(b, "%v\n", &c.check_door)
	fmt.Fscanf(b, "%v\n", &c.can_climb)
	fmt.Fscanf(b, "%v\n", &c.try_left)
	for i := range c.collision_point {
		fmt.Fscanf(b, "%v\n", &c.collision_point[i])
	}
	fmt.Fscanf(b, "%v\n", &c.trophy)
	fmt.Fscanf(b, "%v\n", &c.gun)
	fmt.Fscanf(b, "%v\n", &c.jetpack)
	for _, m := range c.monster {
		fmt.Fscanf(b, "%v\n", &m.typ)
		fmt.Fscanf(b, "%v\n", &m.path_index)
		fmt.Fscanf(b, "%v\n", &m.dead_timer)
		fmt.Fscanf(b, "%v\n", &m.monster_x)
		fmt.Fscanf(b, "%v\n", &m.monster_y)
		fmt.Fscanf(b, "%v\n", &m.monster_px)
		fmt.Fscanf(b, "%v\n", &m.monster_py)
		fmt.Fscanf(b, "%v\n", &m.next_px)
		fmt.Fscanf(b, "%v\n", &m.next_py)
	}

	fmt.Println("Loaded state to slot", c.saveslot)
}
