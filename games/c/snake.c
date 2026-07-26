/*
 * Snake, compiled to freestanding WebAssembly with clang.
 *
 * No libc, no emscripten, no runtime: `clang --target=wasm32 -nostdlib` emits a
 * ~2 KB module whose only exports are the handful of functions below. The whole
 * board lives in static memory; JavaScript reads it straight out of the
 * module's linear memory and paints it onto a canvas.
 */

#define W 24
#define H 24
#define N (W * H)

/* Cell tags, read back by JavaScript. */
#define EMPTY 0
#define BODY 1
#define HEAD 2
#define FOOD 3

/* Bits returned by step(). */
#define ATE 1
#define DIED 2
#define WON 4

#define EXPORT(name) __attribute__((export_name(#name)))

/*
 * With -nostdlib there is no libc to link against, but the optimiser is still
 * free to turn a fill loop into a call to memset. -mbulk-memory normally
 * lowers those to the wasm memory.fill instruction instead; these definitions
 * are the safety net for anything that still comes out as a call.
 */
void *memset(void *dst, int value, unsigned long n) {
  unsigned char *p = dst;
  while (n--) *p++ = (unsigned char)value;
  return dst;
}

void *memcpy(void *dst, const void *src, unsigned long n) {
  unsigned char *d = dst;
  const unsigned char *s = src;
  while (n--) *d++ = *s++;
  return dst;
}

static unsigned char grid[N];
/* Ring buffer of cell indices, head first, walking towards the tail. */
static unsigned short snake[N];
static unsigned int head_idx;
static unsigned int len;
static unsigned int dir;     /* 0 up, 1 right, 2 down, 3 left */
static unsigned int pending;
static unsigned int score_v;
static unsigned int alive_v;
static unsigned int won_v;
static unsigned int rng_state = 0x2545f491u;

/* xorshift32 — deterministic given the seed handed in from JS. */
static unsigned int next_rand(void) {
  unsigned int x = rng_state;
  x ^= x << 13;
  x ^= x >> 17;
  x ^= x << 5;
  rng_state = x;
  return x;
}

static unsigned int tail_cell(void) { return snake[(head_idx + len - 1) % N]; }

/* Place food on a uniformly chosen empty cell. No-op on a full board. */
static void spawn_food(void) {
  unsigned int free_cells = N - len;
  if (free_cells == 0) return;
  unsigned int nth = next_rand() % free_cells;
  for (int i = 0; i < N; i++) {
    if (grid[i] == EMPTY) {
      if (nth == 0) {
        grid[i] = FOOD;
        return;
      }
      nth--;
    }
  }
}

EXPORT(init) void init(unsigned int seed) {
  rng_state = seed ? seed : 0x2545f491u;
  for (int i = 0; i < N; i++) grid[i] = EMPTY;

  head_idx = 0;
  len = 3;
  dir = 1;
  pending = 1;
  score_v = 0;
  alive_v = 1;
  won_v = 0;

  const int row = H / 2;
  for (unsigned int i = 0; i < len; i++) {
    unsigned short cell = (unsigned short)(row * W + (4 - (int)i));
    snake[i] = cell;
    grid[cell] = i == 0 ? HEAD : BODY;
  }

  spawn_food();
}

EXPORT(set_dir) void set_dir(unsigned int d) { pending = d % 4; }

EXPORT(step) unsigned int step(void) {
  if (!alive_v) return DIED;

  /* A reversal would drive the head straight into the neck; ignore it. */
  if (len == 1 || (pending + 2) % 4 != dir) dir = pending;

  unsigned int cur = snake[head_idx];
  int cx = (int)(cur % W), cy = (int)(cur / W);
  int dx = dir == 1 ? 1 : dir == 3 ? -1 : 0;
  int dy = dir == 2 ? 1 : dir == 0 ? -1 : 0;
  int nx = cx + dx, ny = cy + dy;

  if (nx < 0 || ny < 0 || nx >= W || ny >= H) {
    alive_v = 0;
    return DIED;
  }

  unsigned int next = (unsigned int)(ny * W + nx);
  unsigned char cell = grid[next];
  int ate = cell == FOOD;
  /* Chasing your own tail is fine: it moves out of the way this tick. */
  int into_tail = next == tail_cell() && !ate;
  if ((cell == BODY || cell == HEAD) && !into_tail) {
    alive_v = 0;
    return DIED;
  }

  if (!ate) {
    grid[tail_cell()] = EMPTY;
    len--;
  }

  grid[cur] = BODY;
  head_idx = (head_idx + N - 1) % N;
  snake[head_idx] = (unsigned short)next;
  len++;
  grid[next] = HEAD;

  if (ate) {
    score_v += 10;
    if (len == N) {
      alive_v = 0;
      won_v = 1;
      return ATE | WON;
    }
    spawn_food();
    return ATE;
  }

  return 0;
}

EXPORT(score) unsigned int score(void) { return score_v; }
EXPORT(length) unsigned int length(void) { return len; }
EXPORT(alive) unsigned int alive(void) { return alive_v; }
EXPORT(won) unsigned int won(void) { return won_v; }
EXPORT(width) unsigned int width(void) { return W; }
EXPORT(height) unsigned int height(void) { return H; }

/* Pointer to W * H cell tags inside linear memory. */
EXPORT(grid_ptr) unsigned char *grid_ptr(void) { return grid; }
