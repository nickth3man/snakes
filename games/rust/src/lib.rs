//! Snake, as a freestanding WebAssembly module.
//!
//! No `std`, no `wasm-bindgen`, no allocator: the whole game lives in a couple
//! of statics inside linear memory. JavaScript drives the clock, reads the
//! grid straight out of `memory`, and paints it onto a canvas.

#![no_std]

use core::ptr::addr_of_mut;

pub const W: usize = 24;
pub const H: usize = 24;
const N: usize = W * H;

/// Cell tags written into [`Game::grid`], read back by JavaScript.
const EMPTY: u8 = 0;
const BODY: u8 = 1;
const HEAD: u8 = 2;
const FOOD: u8 = 3;

/// Bits returned by [`step`].
const ATE: u32 = 1;
const DIED: u32 = 2;
const WON: u32 = 4;

struct Game {
    grid: [u8; N],
    /// Ring buffer of cell indices, head first, walking towards the tail.
    snake: [u16; N],
    head: usize,
    len: usize,
    dir: u8,
    pending: u8,
    score: u32,
    alive: bool,
    won: bool,
    rng: u32,
}

impl Game {
    const fn new() -> Self {
        Game {
            grid: [EMPTY; N],
            snake: [0; N],
            head: 0,
            len: 0,
            dir: 1,
            pending: 1,
            score: 0,
            alive: false,
            won: false,
            rng: 0x2545_f491,
        }
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

    fn tail(&self) -> u16 {
        self.snake[(self.head + self.len - 1) % N]
    }

    /// Place food on a uniformly chosen empty cell. No-op on a full board.
    fn spawn_food(&mut self) {
        let free = N - self.len;
        if free == 0 {
            return;
        }
        let mut nth = (self.next_rand() as usize) % free;
        for i in 0..N {
            if self.grid[i] == EMPTY {
                if nth == 0 {
                    self.grid[i] = FOOD;
                    return;
                }
                nth -= 1;
            }
        }
    }

    fn start(&mut self, seed: u32) {
        self.rng = if seed == 0 { 0x2545_f491 } else { seed };
        self.grid = [EMPTY; N];
        self.head = 0;
        self.len = 3;
        self.dir = 1;
        self.pending = 1;
        self.score = 0;
        self.alive = true;
        self.won = false;

        let row = H / 2;
        for i in 0..self.len {
            let cell = (row * W + (4 - i)) as u16;
            self.snake[i] = cell;
            self.grid[cell as usize] = if i == 0 { HEAD } else { BODY };
        }

        self.spawn_food();
    }

    fn step(&mut self) -> u32 {
        if !self.alive {
            return DIED;
        }

        // A reversal would drive the head straight into the neck; ignore it.
        if self.len == 1 || (self.pending + 2) % 4 != self.dir {
            self.dir = self.pending;
        }

        let cur = self.snake[self.head] as usize;
        let (cx, cy) = ((cur % W) as i32, (cur / W) as i32);
        let (dx, dy) = match self.dir {
            0 => (0, -1),
            1 => (1, 0),
            2 => (0, 1),
            _ => (-1, 0),
        };
        let (nx, ny) = (cx + dx, cy + dy);

        if nx < 0 || ny < 0 || nx >= W as i32 || ny >= H as i32 {
            self.alive = false;
            return DIED;
        }

        let next = (ny as usize) * W + nx as usize;
        let cell = self.grid[next];
        let ate = cell == FOOD;
        // Chasing your own tail is fine: it moves out of the way this tick.
        let into_tail = next == self.tail() as usize && !ate;
        if (cell == BODY || cell == HEAD) && !into_tail {
            self.alive = false;
            return DIED;
        }

        if !ate {
            let tail = self.tail() as usize;
            self.grid[tail] = EMPTY;
            self.len -= 1;
        }

        self.grid[cur] = BODY;
        self.head = (self.head + N - 1) % N;
        self.snake[self.head] = next as u16;
        self.len += 1;
        self.grid[next] = HEAD;

        if ate {
            self.score += 10;
            if self.len == N {
                self.alive = false;
                self.won = true;
                return ATE | WON;
            }
            self.spawn_food();
            return ATE;
        }

        0
    }
}

static mut STATE: Game = Game::new();

/// `static mut` is the whole point here — there is exactly one game and one
/// thread. `addr_of_mut!` keeps us clear of intermediate references.
#[allow(clippy::mut_from_ref)]
fn game() -> &'static mut Game {
    unsafe { &mut *addr_of_mut!(STATE) }
}

#[no_mangle]
pub extern "C" fn init(seed: u32) {
    game().start(seed);
}

#[no_mangle]
pub extern "C" fn set_dir(dir: u32) {
    game().pending = (dir % 4) as u8;
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
pub extern "C" fn width() -> u32 {
    W as u32
}

#[no_mangle]
pub extern "C" fn height() -> u32 {
    H as u32
}

/// Pointer to `W * H` cell tags inside linear memory.
#[no_mangle]
pub extern "C" fn grid_ptr() -> *const u8 {
    game().grid.as_ptr()
}

#[panic_handler]
fn panic(_info: &core::panic::PanicInfo) -> ! {
    core::arch::wasm32::unreachable()
}
