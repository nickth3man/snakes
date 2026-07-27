/*
 * Snake, compiled to freestanding WebAssembly with clang.
 *
 * No libc, no emscripten, no runtime: `clang --target=wasm32 -nostdlib` emits a
 * module whose only exports are the functions below. The whole board lives in
 * static memory; JavaScript reads it straight out of the module's linear
 * memory and paints it onto a canvas.
 *
 * Both game modes live here. Normal mode consumes a two-deep queue of player
 * directions; demo mode runs the three-tier AI in ai_decide(), which also
 * leaves its reasoning (reachable region, planned path, tier) in buffers the
 * page can draw as an "AI vision" overlay.
 *
 * This is a direct mirror of games/rust/src/lib.rs, down to the exported names.
 */

/* Enough for either orientation: 30x22 landscape, 18x38 portrait. */
#define MAX_CELLS 1600

/* Cell tags, read back by JavaScript. */
#define EMPTY 0
#define BODY 1
#define HEAD 2
#define FOOD 3

/* Bits returned by step(). */
#define ATE 1
#define DIED 2
#define WON 4

/* Tiers returned by ai_decide(), matching the TypeScript controller. */
#define TIER1 0
#define TIER2 1
#define FALLBACK 2

#define NONE 0xFFFFu

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

/* UP, RIGHT, DOWN, LEFT — the order decides ties, so it is load-bearing. */
static const int DX[4] = {0, 1, 0, -1};
static const int DY[4] = {-1, 0, 1, 0};

static int cols_v = 30;
static int rows_v = 22;
static unsigned char grid[MAX_CELLS];
/* Ring buffer of cell indices, head first, walking towards the tail. */
static unsigned short snake[MAX_CELLS];
static int head_idx;
static int len_v;
static int dir_v = 1;
/* Two-deep queue of player directions, drained one per step. */
static int queue[2];
static int queued;
static int food_v = -1;
static unsigned int score_v;
static int alive_v;
static int won_v;
static unsigned int rng_state = 0x2545f491u;

/* What the AI saw on its last decision, for the vision overlay. */
static unsigned int tier_v = FALLBACK;
static unsigned short reachable[MAX_CELLS];
static int reachable_len_v;
static unsigned short path[MAX_CELLS];
static int path_len_v;

/* Head-first copy of the snake, filled on demand by snake_ptr(). */
static unsigned short snapshot[MAX_CELLS];

/* Scratch space for flood fill and breadth-first search. */
static unsigned char seen[MAX_CELLS];
static unsigned short frontier[MAX_CELLS];
static unsigned short parent[MAX_CELLS];
static unsigned char blocked[MAX_CELLS];

static int opposite(int a, int b) { return (a + 2) % 4 == b; }

static int cells(void) { return cols_v * rows_v; }

static int cell_x(int cell) { return cell % cols_v; }
static int cell_y(int cell) { return cell / cols_v; }

/* Returns the cell index, or -1 when off the board. */
static int cell_at(int x, int y) {
  if (x < 0 || y < 0 || x >= cols_v || y >= rows_v) return -1;
  return y * cols_v + x;
}

static int wall_adjacent(int cell) {
  int x = cell_x(cell), y = cell_y(cell);
  return x == 0 || y == 0 || x == cols_v - 1 || y == rows_v - 1;
}

static int iabs(int v) { return v < 0 ? -v : v; }

/* xorshift32 — deterministic given the seed handed in from JS. */
static unsigned int next_rand(void) {
  unsigned int x = rng_state;
  x ^= x << 13;
  x ^= x >> 17;
  x ^= x << 5;
  rng_state = x;
  return x;
}

static int nth_snake(int i) { return snake[(head_idx + i) % MAX_CELLS]; }

static int tail_cell(void) { return nth_snake(len_v - 1); }

/* Place food on a uniformly chosen empty cell; a full board is a win. */
static void spawn_food(void) {
  int free_cells = cells() - len_v;
  if (free_cells == 0) {
    food_v = -1;
    alive_v = 0;
    won_v = 1;
    return;
  }
  unsigned int nth = next_rand() % (unsigned int)free_cells;
  for (int i = 0; i < cells(); i++) {
    if (grid[i] == EMPTY) {
      if (nth == 0) {
        grid[i] = FOOD;
        food_v = i;
        return;
      }
      nth--;
    }
  }
}

EXPORT(init) void init(unsigned int cols, unsigned int rows, unsigned int seed) {
  cols_v = (int)cols;
  rows_v = (int)rows;
  if (cols_v < 4) cols_v = 4;
  if (rows_v < 4) rows_v = 4;
  if (cols_v > 40) cols_v = 40;
  if (rows_v > 40) rows_v = 40;
  if (cells() > MAX_CELLS) {
    cols_v = 30;
    rows_v = 22;
  }
  rng_state = seed ? seed : 0x2545f491u;

  for (int i = 0; i < MAX_CELLS; i++) grid[i] = EMPTY;
  head_idx = 0;
  len_v = 3;
  dir_v = 1;
  queued = 0;
  score_v = 0;
  alive_v = 1;
  won_v = 0;
  tier_v = FALLBACK;
  reachable_len_v = 0;
  path_len_v = 0;

  /* Same opening as the TypeScript engine: three cells, mid-row, facing right. */
  int mid = rows_v / 2;
  int start_col = cols_v / 2 < 5 ? cols_v / 2 : 5;
  int safe = start_col > 2 ? start_col : 2;
  for (int i = 0; i < len_v; i++) {
    int cell = mid * cols_v + (safe - i);
    snake[i] = (unsigned short)cell;
    grid[cell] = i == 0 ? HEAD : BODY;
  }

  spawn_food();
}

/* Queue a player direction, mirroring the scene's two-deep input buffer. */
EXPORT(queue_dir) void queue_dir(unsigned int dir) {
  int d = (int)(dir % 4);
  if (queued >= 2) return;
  int last = queued > 0 ? queue[queued - 1] : dir_v;
  if (opposite(d, last)) return;
  queue[queued++] = d;
}

static int take_queued(void) {
  if (queued == 0) return dir_v;
  int next = queue[0];
  queue[0] = queue[1];
  queued--;
  return next;
}

EXPORT(step) unsigned int step(void) {
  if (!alive_v) return DIED;

  int want = take_queued();
  /* A reversal would drive the head straight into the neck; ignore it. */
  if (!(len_v > 1 && opposite(want, dir_v))) dir_v = want;

  int cur = nth_snake(0);
  int next = cell_at(cell_x(cur) + DX[dir_v], cell_y(cur) + DY[dir_v]);
  if (next < 0) {
    alive_v = 0;
    return DIED;
  }

  /* Self-collision is checked against the whole body, tail included — the
     TypeScript engine does the same before the tail is popped. */
  if (grid[next] == BODY || grid[next] == HEAD) {
    alive_v = 0;
    return DIED;
  }

  int ate = next == food_v;
  if (!ate) {
    grid[tail_cell()] = EMPTY;
    len_v--;
  }

  grid[cur] = BODY;
  head_idx = (head_idx + MAX_CELLS - 1) % MAX_CELLS;
  snake[head_idx] = (unsigned short)next;
  len_v++;
  grid[next] = HEAD;

  if (ate) {
    score_v++;
    spawn_food();
    return won_v ? (ATE | WON) : ATE;
  }

  return 0;
}

/* ──────────── Demo-mode AI ──────────── */

/* Mark every snake cell except the tail: after one step the tail moves on. */
static void mark_body_minus_tail(void) {
  for (int i = 0; i < cells(); i++) blocked[i] = 0;
  for (int i = 0; i < len_v - 1; i++) blocked[nth_snake(i)] = 1;
}

/*
 * Breadth-first search from start to goal, writing the path (inclusive of both
 * ends) into path[]. Returns the path length, or 0 if unreachable.
 */
static int bfs(int start, int goal) {
  if (blocked[start]) return 0;
  if (start == goal) {
    path[0] = (unsigned short)start;
    return 1;
  }

  for (int i = 0; i < cells(); i++) {
    seen[i] = 0;
    parent[i] = NONE;
  }
  seen[start] = 1;
  frontier[0] = (unsigned short)start;
  int head = 0, tail = 1;

  while (head < tail) {
    int pos = frontier[head++];
    for (int d = 0; d < 4; d++) {
      int next = cell_at(cell_x(pos) + DX[d], cell_y(pos) + DY[d]);
      if (next < 0 || blocked[next] || seen[next]) continue;
      parent[next] = (unsigned short)pos;
      if (next == goal) {
        /* Walk the parents back, then reverse into place. */
        int n = 0, cur = goal;
        for (;;) {
          frontier[n++] = (unsigned short)cur;
          if (cur == start) break;
          cur = parent[cur];
        }
        for (int i = 0; i < n; i++) path[i] = frontier[n - 1 - i];
        return n;
      }
      seen[next] = 1;
      frontier[tail++] = (unsigned short)next;
    }
  }

  return 0;
}

/* Flood fill from start, writing reachable cells into reachable[]. */
static int flood(int start) {
  if (blocked[start]) return 0;
  for (int i = 0; i < cells(); i++) seen[i] = 0;
  seen[start] = 1;
  reachable[0] = (unsigned short)start;
  int head = 0, tail = 1;

  while (head < tail) {
    int pos = reachable[head++];
    for (int d = 0; d < 4; d++) {
      int next = cell_at(cell_x(pos) + DX[d], cell_y(pos) + DY[d]);
      if (next < 0 || blocked[next] || seen[next]) continue;
      seen[next] = 1;
      reachable[tail++] = (unsigned short)next;
    }
  }

  return tail;
}

static int path_turns(int len) {
  if (len < 3) return 0;
  int turns = 0;
  int pdx = cell_x(path[1]) - cell_x(path[0]);
  int pdy = cell_y(path[1]) - cell_y(path[0]);
  for (int i = 2; i < len; i++) {
    int dx = cell_x(path[i]) - cell_x(path[i - 1]);
    int dy = cell_y(path[i]) - cell_y(path[i - 1]);
    if (dx != pdx || dy != pdy) {
      turns++;
      pdx = dx;
      pdy = dy;
    }
  }
  return turns;
}

static int path_wall_hugs(int len) {
  int hugs = 0;
  for (int i = 0; i < len; i++) {
    if (wall_adjacent(path[i])) hugs++;
  }
  return hugs;
}

/* The AI bypasses the player queue: its choice is the next move, period. */
static void dir_from_ai(int dir) {
  queue[0] = dir;
  queued = 1;
}

/*
 * Pick a direction the way the TypeScript demo controller does, and leave the
 * reasoning behind for the vision overlay.
 */
EXPORT(ai_decide) unsigned int ai_decide(void) {
  int head = nth_snake(0);
  int hx = cell_x(head), hy = cell_y(head);

  /* Step 1: immediate moves that are not reversals, walls or body cells. */
  int move_dir[4], move_cell[4], move_count = 0;
  for (int d = 0; d < 4; d++) {
    if (len_v > 1 && opposite(d, dir_v)) continue;
    int next = cell_at(hx + DX[d], hy + DY[d]);
    if (next < 0) continue;
    if (grid[next] == BODY || grid[next] == HEAD) continue;
    move_dir[move_count] = d;
    move_cell[move_count] = next;
    move_count++;
  }

  mark_body_minus_tail();
  path_len_v = 0;
  reachable_len_v = 0;

  if (move_count == 0) {
    tier_v = FALLBACK;
    return FALLBACK;
  }

  if (move_count == 1) {
    dir_from_ai(move_dir[0]);
    reachable_len_v = flood(move_cell[0]);
    tier_v = FALLBACK;
    return FALLBACK;
  }

  /* Step 2: tier 1 — reach the food while keeping room to breathe. */
  if (food_v >= 0) {
    int best = -1, best_len = 0, best_hugs = 0, best_turns = 0;
    for (int i = 0; i < move_count; i++) {
      int plen = bfs(move_cell[i], food_v);
      if (plen == 0) continue;
      int turns = path_turns(plen);
      int hugs = path_wall_hugs(plen);
      int space = flood(move_cell[i]);
      /* space < len * 1.2, in integers. */
      if (space * 5 < len_v * 6) continue;
      int better = best < 0 || plen < best_len ||
                   (plen == best_len &&
                    (hugs > best_hugs || (hugs == best_hugs && turns > best_turns)));
      if (better) {
        best = i;
        best_len = plen;
        best_hugs = hugs;
        best_turns = turns;
      }
    }

    if (best >= 0) {
      dir_from_ai(move_dir[best]);
      /* Redo the winner's work so the buffers describe the move taken. */
      int plen = bfs(move_cell[best], food_v);
      /* The overlay draws from the current head, so shift the path along. */
      if (plen > 0 && plen + 1 <= MAX_CELLS) {
        for (int i = plen; i > 0; i--) path[i] = path[i - 1];
        path[0] = (unsigned short)head;
        path_len_v = plen + 1;
      }
      reachable_len_v = flood(move_cell[best]);
      tier_v = TIER1;
      return TIER1;
    }
  }

  /* Step 3: tier 2 — head for the most open space. */
  {
    int best = -1, best_space = 0, best_hug = 0, best_dist = 0;
    for (int i = 0; i < move_count; i++) {
      int space = flood(move_cell[i]);
      int hug = wall_adjacent(move_cell[i]) ? 1 : 0;
      int dist = 0;
      if (food_v >= 0) {
        dist = iabs(cell_x(food_v) - cell_x(move_cell[i])) +
               iabs(cell_y(food_v) - cell_y(move_cell[i]));
      }
      int better = best < 0 || space > best_space ||
                   (space == best_space &&
                    (hug > best_hug || (hug == best_hug && dist < best_dist)));
      if (better) {
        best = i;
        best_space = space;
        best_hug = hug;
        best_dist = dist;
      }
    }

    if (best >= 0 && best_space > 1) {
      dir_from_ai(move_dir[best]);
      reachable_len_v = flood(move_cell[best]);
      tier_v = TIER2;
      return TIER2;
    }
  }

  /* Step 4: fallback — lunge at the food and hope. */
  int chosen = 0;
  if (food_v >= 0) {
    int best_dist = 0x7fffffff;
    for (int i = 0; i < move_count; i++) {
      int dist = iabs(cell_x(food_v) - cell_x(move_cell[i])) +
                 iabs(cell_y(food_v) - cell_y(move_cell[i]));
      if (dist < best_dist) {
        best_dist = dist;
        chosen = i;
      }
    }
  }
  dir_from_ai(move_dir[chosen]);
  reachable_len_v = flood(move_cell[chosen]);
  tier_v = FALLBACK;
  return FALLBACK;
}

/* ──────────── Exports read by the page ──────────── */

EXPORT(score) unsigned int score(void) { return score_v; }
EXPORT(length) unsigned int length(void) { return (unsigned int)len_v; }
EXPORT(alive) unsigned int alive(void) { return (unsigned int)alive_v; }
EXPORT(won) unsigned int won(void) { return (unsigned int)won_v; }
EXPORT(direction) unsigned int direction(void) { return (unsigned int)dir_v; }
EXPORT(food_cell) int food_cell(void) { return food_v; }
EXPORT(width) unsigned int width(void) { return (unsigned int)cols_v; }
EXPORT(height) unsigned int height(void) { return (unsigned int)rows_v; }

/* Pointer to width() * height() cell tags inside linear memory. */
EXPORT(grid_ptr) unsigned char *grid_ptr(void) { return grid; }

/* Copies the snake into a contiguous buffer, head first, and returns it.
   length() says how many entries are valid. */
EXPORT(snake_ptr) unsigned short *snake_ptr(void) {
  for (int i = 0; i < len_v; i++) snapshot[i] = (unsigned short)nth_snake(i);
  return snapshot;
}

EXPORT(ai_tier) unsigned int ai_tier(void) { return tier_v; }
EXPORT(ai_reachable_ptr) unsigned short *ai_reachable_ptr(void) { return reachable; }
EXPORT(ai_reachable_len) unsigned int ai_reachable_len(void) {
  return (unsigned int)reachable_len_v;
}
EXPORT(ai_path_ptr) unsigned short *ai_path_ptr(void) { return path; }
EXPORT(ai_path_len) unsigned int ai_path_len(void) { return (unsigned int)path_len_v; }
