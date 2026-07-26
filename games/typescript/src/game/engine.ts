export interface Point {
  x: number;
  y: number;
}

export type RNG = () => number;

function cellKey(x: number, y: number): string {
  return `${x},${y}`;
}

function parseKey(key: string): Point {
  const parts = key.split(",");
  return { x: parseInt(parts[0], 10), y: parseInt(parts[1], 10) };
}

function isOpposite(a: Point, b: Point): boolean {
  return a.x === -b.x && a.y === -b.y;
}

export class SnakeGame {
  private cols: number;
  private rows: number;
  private snake: Point[] = [];
  private food: Point | null = null;
  private direction: Point = { x: 1, y: 0 };
  private score = 0;
  private alive = false;
  private won = false;

  private rng: RNG;

  /** Free-cell set optimization (L-03). Keys are "x,y" strings. */
  private freeCells: Set<string> = new Set();

  constructor(config: { cols: number; rows: number; rng?: RNG }) {
    this.cols = config.cols;
    this.rows = config.rows;
    this.rng = config.rng ?? Math.random;
  }

  /** Start or restart a game. Resets snake, score, food, alive/won. */
  start(): void {
    const midRow = Math.floor(this.rows / 2);
    const startCol = Math.min(Math.floor(this.cols / 2), 5);
    const safe = Math.max(startCol, 2);

    this.snake = [
      { x: safe, y: midRow },
      { x: safe - 1, y: midRow },
      { x: safe - 2, y: midRow },
    ];
    this.direction = { x: 1, y: 0 };
    this.alive = true;
    this.won = false;
    this.score = 0;

    // Initialize free-cell set
    this.freeCells.clear();
    for (let x = 0; x < this.cols; x++) {
      for (let y = 0; y < this.rows; y++) {
        this.freeCells.add(cellKey(x, y));
      }
    }
    for (const seg of this.snake) {
      this.freeCells.delete(cellKey(seg.x, seg.y));
    }

    this.spawnFood();
  }

  /**
   * Process one tick with the given direction.
   * Returns what happened this tick.
   */
  step(direction: Point): { ate: boolean; died: boolean; won: boolean } {
    if (!this.alive) {
      return { ate: false, died: false, won: this.won };
    }

    // Reversal guard: ignore exact opposite direction
    const dir = isOpposite(direction, this.direction)
      ? this.direction
      : direction;
    this.direction = dir;

    const head = this.snake[0];
    const newHead: Point = {
      x: head.x + dir.x,
      y: head.y + dir.y,
    };

    // Wall death
    if (
      newHead.x < 0 ||
      newHead.x >= this.cols ||
      newHead.y < 0 ||
      newHead.y >= this.rows
    ) {
      this.alive = false;
      return { ate: false, died: true, won: false };
    }

    // Self death: check against full body before popping tail
    if (
      this.snake.some(
        (seg) => seg.x === newHead.x && seg.y === newHead.y,
      )
    ) {
      this.alive = false;
      return { ate: false, died: true, won: false };
    }

    // Move snake forward
    this.snake.unshift(newHead);
    this.freeCells.delete(cellKey(newHead.x, newHead.y));

    // Check food
    const ate =
      this.food !== null &&
      this.food.x === newHead.x &&
      this.food.y === newHead.y;

    if (ate) {
      this.score++;
      this.spawnFood(); // may set won = true
      // Don't pop tail — snake grows by 1
    } else {
      const tail = this.snake.pop()!;
      this.freeCells.add(cellKey(tail.x, tail.y));
    }

    return { ate, died: false, won: this.won };
  }

  /** Read-only snapshot of current state (plain objects, not references). */
  getState(): {
    snake: Point[];
    food: Point | null;
    direction: Point;
    score: number;
    alive: boolean;
    won: boolean;
  } {
    return {
      snake: this.snake.map((s) => ({ x: s.x, y: s.y })),
      food: this.food ? { x: this.food.x, y: this.food.y } : null,
      direction: { x: this.direction.x, y: this.direction.y },
      score: this.score,
      alive: this.alive,
      won: this.won,
    };
  }

  /** Place food on a random free cell. Sets won=true when board is full. */
  private spawnFood(): void {
    if (this.freeCells.size === 0) {
      this.food = null;
      this.alive = false;
      this.won = true;
      return;
    }

    const free = Array.from(this.freeCells);
    const key = free[Math.floor(this.rng() * free.length)];
    this.food = parseKey(key);
  }
}
