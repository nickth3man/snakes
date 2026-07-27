//! Snake, as a freestanding WebAssembly module.
//!
//! No `std`, no `wasm-bindgen`, no allocator: the whole game lives in a handful
//! of statics inside linear memory. JavaScript drives the clock, reads the
//! board straight out of `memory`, and paints it onto a canvas.
//!
//! Both game modes live here. Normal mode consumes a two-deep queue of player
//! directions; demo mode runs the three-tier AI in [`ai_decide`], which also
//! leaves its reasoning (reachable region, planned path, tier) in buffers the
//! page can draw as an "AI vision" overlay.

#![no_std]

use core::ptr::addr_of_mut;

/// Enough for either orientation: 30x22 landscape, 18x38 portrait.
const MAX_CELLS: usize = 1600;

/// Cell tags written into [`Game::grid`], read back by JavaScript.
const EMPTY: u8 = 0;
const BODY: u8 = 1;
const HEAD: u8 = 2;
const FOOD: u8 = 3;

/// Bits returned by [`step`].
const ATE: u32 = 1;
const DIED: u32 = 2;
const WON: u32 = 4;

/// Tiers returned by [`ai_decide`], matching the TypeScript controller.
const TIER1: u32 = 0;
const TIER2: u32 = 1;
const FALLBACK: u32 = 2;

const NONE: u16 = u16::MAX;

/// UP, RIGHT, DOWN, LEFT — the order decides ties, so it is load-bearing.
const DELTA: [(i32, i32); 4] = [(0, -1), (1, 0), (0, 1), (-1, 0)];

fn opposite(a: u8, b: u8) -> bool {
    (a + 2) % 4 == b
}

struct Game {
    cols: usize,
    rows: usize,
    grid: [u8; MAX_CELLS],
    /// Ring buffer of cell indices, head first, walking towards the tail.
    snake: [u16; MAX_CELLS],
    head: usize,
    len: usize,
    dir: u8,
    /// Two-deep queue of player directions, drained one per step.
    queue: [u8; 2],
    queued: usize,
    food: i32,
    score: u32,
    alive: bool,
    won: bool,
    rng: u32,

    /// What the AI saw on its last decision, for the vision overlay.
    tier: u32,
    reachable: [u16; MAX_CELLS],
    reachable_len: usize,
    path: [u16; MAX_CELLS],
    path_len: usize,

    /// Head-first copy of the snake, filled on demand by [`snake_ptr`].
    snapshot: [u16; MAX_CELLS],

    /// Scratch space for flood fill and breadth-first search.
    seen: [u8; MAX_CELLS],
    frontier: [u16; MAX_CELLS],
    parent: [u16; MAX_CELLS],
    blocked: [u8; MAX_CELLS],
}

impl Game {
    const fn new() -> Self {
        Game {
            cols: 30,
            rows: 22,
            grid: [EMPTY; MAX_CELLS],
            snake: [0; MAX_CELLS],
            head: 0,
            len: 0,
            dir: 1,
            queue: [1; 2],
            queued: 0,
            food: -1,
            score: 0,
            alive: false,
            won: false,
            rng: 0x2545_f491,
            tier: FALLBACK,
            reachable: [0; MAX_CELLS],
            reachable_len: 0,
            path: [0; MAX_CELLS],
            path_len: 0,
            snapshot: [0; MAX_CELLS],
            seen: [0; MAX_CELLS],
            frontier: [0; MAX_CELLS],
            // Reset per search, so zero-init keeps this out of the data segment.
            parent: [0; MAX_CELLS],
            blocked: [0; MAX_CELLS],
        }
    }

    fn cells(&self) -> usize {
        self.cols * self.rows
    }

    fn xy(&self, cell: usize) -> (i32, i32) {
        ((cell % self.cols) as i32, (cell / self.cols) as i32)
    }

    fn cell_at(&self, x: i32, y: i32) -> Option<usize> {
        if x < 0 || y < 0 || x >= self.cols as i32 || y >= self.rows as i32 {
            None
        } else {
            Some(y as usize * self.cols + x as usize)
        }
    }

    fn wall_adjacent(&self, cell: usize) -> bool {
        let (x, y) = self.xy(cell);
        x == 0 || y == 0 || x == self.cols as i32 - 1 || y == self.rows as i32 - 1
    }

    /// xorshift32 — deterministic given the seed handed in from JS.
    fn next_rand(&mut self) -> u32 {
        let mut x = self.rng;
        x ^= x << 13;
        x ^= x >> 17;
        x ^= x << 5;
        self.rng = x;
        x
    }

    fn nth_snake(&self, i: usize) -> usize {
        self.snake[(self.head + i) % MAX_CELLS] as usize
    }

    fn tail(&self) -> usize {
        self.nth_snake(self.len - 1)
    }

    /// Place food on a uniformly chosen empty cell; a full board is a win.
    fn spawn_food(&mut self) {
        let free = self.cells() - self.len;
        if free == 0 {
            self.food = -1;
            self.alive = false;
            self.won = true;
            return;
        }
        let mut nth = (self.next_rand() as usize) % free;
        for i in 0..self.cells() {
            if self.grid[i] == EMPTY {
                if nth == 0 {
                    self.grid[i] = FOOD;
                    self.food = i as i32;
                    return;
                }
                nth -= 1;
            }
        }
    }

    fn start(&mut self, cols: usize, rows: usize, seed: u32) {
        self.cols = cols.clamp(4, 40);
        self.rows = rows.clamp(4, 40);
        if self.cells() > MAX_CELLS {
            self.cols = 30;
            self.rows = 22;
        }
        self.rng = if seed == 0 { 0x2545_f491 } else { seed };

        self.grid = [EMPTY; MAX_CELLS];
        self.head = 0;
        self.len = 3;
        self.dir = 1;
        self.queued = 0;
        self.score = 0;
        self.alive = true;
        self.won = false;
        self.tier = FALLBACK;
        self.reachable_len = 0;
        self.path_len = 0;

        // Same opening as the TypeScript engine: three cells, mid-row, facing right.
        let mid = self.rows / 2;
        let start_col = if self.cols / 2 < 5 { self.cols / 2 } else { 5 };
        let safe = if start_col > 2 { start_col } else { 2 };
        for i in 0..self.len {
            let cell = mid * self.cols + (safe - i);
            self.snake[i] = cell as u16;
            self.grid[cell] = if i == 0 { HEAD } else { BODY };
        }

        self.spawn_food();
    }

    /// Queue a player direction, mirroring the scene's two-deep input buffer.
    fn queue_dir(&mut self, dir: u8) {
        if self.queued >= 2 {
            return;
        }
        let last = if self.queued > 0 {
            self.queue[self.queued - 1]
        } else {
            self.dir
        };
        if opposite(dir, last) {
            return;
        }
        self.queue[self.queued] = dir;
        self.queued += 1;
    }

    fn take_queued(&mut self) -> u8 {
        if self.queued == 0 {
            return self.dir;
        }
        let next = self.queue[0];
        self.queue[0] = self.queue[1];
        self.queued -= 1;
        next
    }

    fn step(&mut self) -> u32 {
        if !self.alive {
            return DIED;
        }

        let want = self.take_queued();
        // A reversal would drive the head straight into the neck; ignore it.
        if !(self.len > 1 && opposite(want, self.dir)) {
            self.dir = want;
        }

        let cur = self.nth_snake(0);
        let (cx, cy) = self.xy(cur);
        let (dx, dy) = DELTA[self.dir as usize];

        let next = match self.cell_at(cx + dx, cy + dy) {
            Some(c) => c,
            None => {
                self.alive = false;
                return DIED;
            }
        };

        // Self-collision is checked against the whole body, tail included —
        // the TypeScript engine does the same before the tail is popped.
        if self.grid[next] == BODY || self.grid[next] == HEAD {
            self.alive = false;
            return DIED;
        }

        let ate = next as i32 == self.food;
        if !ate {
            let tail = self.tail();
            self.grid[tail] = EMPTY;
            self.len -= 1;
        }

        self.grid[cur] = BODY;
        self.head = (self.head + MAX_CELLS - 1) % MAX_CELLS;
        self.snake[self.head] = next as u16;
        self.len += 1;
        self.grid[next] = HEAD;

        if ate {
            self.score += 1;
            self.spawn_food();
            return if self.won { ATE | WON } else { ATE };
        }

        0
    }

    /* ──────────── Demo-mode AI ──────────── */

    /// Mark every snake cell except the tail: after one step the tail moves on.
    fn mark_body_minus_tail(&mut self) {
        for i in 0..self.cells() {
            self.blocked[i] = 0;
        }
        for i in 0..self.len - 1 {
            let cell = self.nth_snake(i);
            self.blocked[cell] = 1;
        }
    }

    /// Breadth-first search from `start` to `goal`, writing the path (inclusive
    /// of both ends) into `self.path`. Returns the path length, or 0 if
    /// unreachable.
    fn bfs(&mut self, start: usize, goal: usize) -> usize {
        if self.blocked[start] == 1 {
            return 0;
        }
        if start == goal {
            self.path[0] = start as u16;
            return 1;
        }

        for i in 0..self.cells() {
            self.seen[i] = 0;
            self.parent[i] = NONE;
        }
        self.seen[start] = 1;
        self.frontier[0] = start as u16;
        let (mut head, mut tail) = (0usize, 1usize);

        while head < tail {
            let pos = self.frontier[head] as usize;
            head += 1;
            let (px, py) = self.xy(pos);
            for &(dx, dy) in DELTA.iter() {
                let next = match self.cell_at(px + dx, py + dy) {
                    Some(c) => c,
                    None => continue,
                };
                if self.blocked[next] == 1 || self.seen[next] == 1 {
                    continue;
                }
                self.parent[next] = pos as u16;
                if next == goal {
                    // Walk the parents back, then reverse into place.
                    let mut n = 0;
                    let mut cur = goal;
                    loop {
                        self.frontier[n] = cur as u16;
                        n += 1;
                        if cur == start {
                            break;
                        }
                        cur = self.parent[cur] as usize;
                    }
                    for i in 0..n {
                        self.path[i] = self.frontier[n - 1 - i];
                    }
                    return n;
                }
                self.seen[next] = 1;
                self.frontier[tail] = next as u16;
                tail += 1;
            }
        }

        0
    }

    /// Flood fill from `start`, writing reachable cells into `self.reachable`.
    fn flood(&mut self, start: usize) -> usize {
        if self.blocked[start] == 1 {
            return 0;
        }
        for i in 0..self.cells() {
            self.seen[i] = 0;
        }
        self.seen[start] = 1;
        self.reachable[0] = start as u16;
        let (mut head, mut tail) = (0usize, 1usize);

        while head < tail {
            let pos = self.reachable[head] as usize;
            head += 1;
            let (px, py) = self.xy(pos);
            for &(dx, dy) in DELTA.iter() {
                let next = match self.cell_at(px + dx, py + dy) {
                    Some(c) => c,
                    None => continue,
                };
                if self.blocked[next] == 1 || self.seen[next] == 1 {
                    continue;
                }
                self.seen[next] = 1;
                self.reachable[tail] = next as u16;
                tail += 1;
            }
        }

        tail
    }

    fn path_turns(&self, len: usize) -> u32 {
        if len < 3 {
            return 0;
        }
        let mut turns = 0;
        let (mut pdx, mut pdy) = {
            let (ax, ay) = self.xy(self.path[0] as usize);
            let (bx, by) = self.xy(self.path[1] as usize);
            (bx - ax, by - ay)
        };
        for i in 2..len {
            let (ax, ay) = self.xy(self.path[i - 1] as usize);
            let (bx, by) = self.xy(self.path[i] as usize);
            let (dx, dy) = (bx - ax, by - ay);
            if dx != pdx || dy != pdy {
                turns += 1;
                pdx = dx;
                pdy = dy;
            }
        }
        turns
    }

    fn path_wall_hugs(&self, len: usize) -> u32 {
        let mut hugs = 0;
        for i in 0..len {
            if self.wall_adjacent(self.path[i] as usize) {
                hugs += 1;
            }
        }
        hugs
    }

    /// Pick a direction the way the TypeScript demo controller does, and leave
    /// the reasoning behind for the vision overlay.
    fn ai_decide(&mut self) -> u32 {
        let head = self.nth_snake(0);
        let (hx, hy) = self.xy(head);

        // Step 1: immediate moves that are not reversals, walls or body cells.
        let mut moves: [(u8, usize); 4] = [(0, 0); 4];
        let mut move_count = 0;
        for d in 0..4u8 {
            if self.len > 1 && opposite(d, self.dir) {
                continue;
            }
            let (dx, dy) = DELTA[d as usize];
            let next = match self.cell_at(hx + dx, hy + dy) {
                Some(c) => c,
                None => continue,
            };
            if self.grid[next] == BODY || self.grid[next] == HEAD {
                continue;
            }
            moves[move_count] = (d, next);
            move_count += 1;
        }

        self.mark_body_minus_tail();
        self.path_len = 0;
        self.reachable_len = 0;

        if move_count == 0 {
            self.tier = FALLBACK;
            return FALLBACK;
        }

        if move_count == 1 {
            let (dir, next) = moves[0];
            self.dir_from_ai(dir);
            self.reachable_len = self.flood(next);
            self.tier = FALLBACK;
            return FALLBACK;
        }

        // Step 2: tier 1 — reach the food while keeping room to breathe.
        if self.food >= 0 {
            let food = self.food as usize;
            let mut best: Option<(u8, usize, usize, u32, u32)> = None;
            for i in 0..move_count {
                let (dir, next) = moves[i];
                let path_len = self.bfs(next, food);
                if path_len == 0 {
                    continue;
                }
                let turns = self.path_turns(path_len);
                let hugs = self.path_wall_hugs(path_len);
                let space = self.flood(next);
                // space < len * 1.2, in integers.
                if space * 5 < self.len * 6 {
                    continue;
                }
                let better = match best {
                    None => true,
                    Some((_, _, blen, bhugs, bturns)) => {
                        path_len < blen
                            || (path_len == blen
                                && (hugs > bhugs || (hugs == bhugs && turns > bturns)))
                    }
                };
                if better {
                    best = Some((dir, next, path_len, hugs, turns));
                }
            }

            if let Some((dir, next, _, _, _)) = best {
                self.dir_from_ai(dir);
                // Redo the winner's work so the buffers describe the move taken.
                let path_len = self.bfs(next, food);
                // The overlay draws from the current head, so shift the path along.
                if path_len > 0 && path_len + 1 <= MAX_CELLS {
                    let mut i = path_len;
                    while i > 0 {
                        self.path[i] = self.path[i - 1];
                        i -= 1;
                    }
                    self.path[0] = head as u16;
                    self.path_len = path_len + 1;
                }
                self.reachable_len = self.flood(next);
                self.tier = TIER1;
                return TIER1;
            }
        }

        // Step 3: tier 2 — head for the most open space.
        {
            let mut best: Option<(u8, usize, usize, u32, i32)> = None;
            for i in 0..move_count {
                let (dir, next) = moves[i];
                let space = self.flood(next);
                let hug = if self.wall_adjacent(next) { 1u32 } else { 0 };
                let dist = if self.food >= 0 {
                    let (fx, fy) = self.xy(self.food as usize);
                    let (nx, ny) = self.xy(next);
                    (fx - nx).abs() + (fy - ny).abs()
                } else {
                    0
                };
                let better = match best {
                    None => true,
                    Some((_, _, bspace, bhug, bdist)) => {
                        space > bspace
                            || (space == bspace
                                && (hug > bhug || (hug == bhug && dist < bdist)))
                    }
                };
                if better {
                    best = Some((dir, next, space, hug, dist));
                }
            }

            if let Some((dir, next, space, _, _)) = best {
                if space > 1 {
                    self.dir_from_ai(dir);
                    self.reachable_len = self.flood(next);
                    self.tier = TIER2;
                    return TIER2;
                }
            }
        }

        // Step 4: fallback — lunge at the food and hope.
        let mut chosen = moves[0];
        if self.food >= 0 {
            let (fx, fy) = self.xy(self.food as usize);
            let mut best_dist = i32::MAX;
            for i in 0..move_count {
                let (nx, ny) = self.xy(moves[i].1);
                let dist = (fx - nx).abs() + (fy - ny).abs();
                if dist < best_dist {
                    best_dist = dist;
                    chosen = moves[i];
                }
            }
        }
        self.dir_from_ai(chosen.0);
        self.reachable_len = self.flood(chosen.1);
        self.tier = FALLBACK;
        FALLBACK
    }

    /// The AI bypasses the player queue: its choice is the next move, period.
    fn dir_from_ai(&mut self, dir: u8) {
        self.queued = 0;
        self.queue[0] = dir;
        self.queued = 1;
    }
}

static mut STATE: Game = Game::new();

/// `static mut` is the whole point here — there is exactly one game and one
/// thread. `addr_of_mut!` keeps us clear of intermediate references.
fn game() -> &'static mut Game {
    unsafe { &mut *addr_of_mut!(STATE) }
}

#[no_mangle]
pub extern "C" fn init(cols: u32, rows: u32, seed: u32) {
    game().start(cols as usize, rows as usize, seed);
}

#[no_mangle]
pub extern "C" fn queue_dir(dir: u32) {
    game().queue_dir((dir % 4) as u8);
}

#[no_mangle]
pub extern "C" fn ai_decide() -> u32 {
    game().ai_decide()
}

#[no_mangle]
pub extern "C" fn step() -> u32 {
    game().step()
}

#[no_mangle]
pub extern "C" fn score() -> u32 {
    game().score
}

#[no_mangle]
pub extern "C" fn length() -> u32 {
    game().len as u32
}

#[no_mangle]
pub extern "C" fn alive() -> u32 {
    game().alive as u32
}

#[no_mangle]
pub extern "C" fn won() -> u32 {
    game().won as u32
}

#[no_mangle]
pub extern "C" fn direction() -> u32 {
    game().dir as u32
}

#[no_mangle]
pub extern "C" fn food_cell() -> i32 {
    game().food
}

#[no_mangle]
pub extern "C" fn width() -> u32 {
    game().cols as u32
}

#[no_mangle]
pub extern "C" fn height() -> u32 {
    game().rows as u32
}

/// Pointer to `width() * height()` cell tags inside linear memory.
#[no_mangle]
pub extern "C" fn grid_ptr() -> *const u8 {
    game().grid.as_ptr()
}

/// Copies the snake into a contiguous buffer, head first, and returns it.
/// `length()` says how many entries are valid.
#[no_mangle]
pub extern "C" fn snake_ptr() -> *const u16 {
    let g = game();
    for i in 0..g.len {
        g.snapshot[i] = g.nth_snake(i) as u16;
    }
    g.snapshot.as_ptr()
}

#[no_mangle]
pub extern "C" fn ai_tier() -> u32 {
    game().tier
}

#[no_mangle]
pub extern "C" fn ai_reachable_ptr() -> *const u16 {
    game().reachable.as_ptr()
}

#[no_mangle]
pub extern "C" fn ai_reachable_len() -> u32 {
    game().reachable_len as u32
}

#[no_mangle]
pub extern "C" fn ai_path_ptr() -> *const u16 {
    game().path.as_ptr()
}

#[no_mangle]
pub extern "C" fn ai_path_len() -> u32 {
    game().path_len as u32
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    core::arch::wasm32::unreachable()
}
