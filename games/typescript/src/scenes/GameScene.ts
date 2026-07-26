import Phaser from "phaser";
import { COLS, ROWS, COLS_PORTRAIT, ROWS_PORTRAIT, WALL_COLOR, GRID_COLOR, SNAKE_HEAD_COLOR, SNAKE_BODY_COLOR, FOOD_COLOR, SNAKE_EYE_COLOR } from "../config";
import { SnakeGame, Point } from "../game/engine";
import { getAIDirection, AIDebugInfo } from "../ai/demo-controller";
import { TouchControls } from "../input/TouchControls";
import { computeLayout, Layout } from "../layout";

export class GameScene extends Phaser.Scene {
  private engine!: SnakeGame;
  private directionQueue: Point[] = [];
  private moveTimer = 0;
  private moveInterval = 130;
  private mode: "normal" | "demo" = "normal";
  private layout!: Layout;

  private scoreText!: Phaser.GameObjects.Text;
  private bestScore = 0;
  private bestScoreText!: Phaser.GameObjects.Text;
  private gameOverText!: Phaser.GameObjects.Text;
  private graphics!: Phaser.GameObjects.Graphics;
  private cursors!: Phaser.Types.Input.Keyboard.CursorKeys;

  private headRect!: Phaser.GameObjects.Rectangle;
  private headGlow!: Phaser.GameObjects.Rectangle;
  private eyeLeft!: Phaser.GameObjects.Arc;
  private eyeRight!: Phaser.GameObjects.Arc;
  private foodCircle!: Phaser.GameObjects.Arc;
  private lastFoodKey: string | null = null;
  private lastDisplayedScore = 0;

  private aiDebug: AIDebugInfo | null = null;
  private showAIDebug = false;
  private tierBadge!: Phaser.GameObjects.Container;
  private tierBadgeText!: Phaser.GameObjects.Text;
  private tierBadgePill!: Phaser.GameObjects.Graphics;
  private lastTier: string | null = null;
  private tierToastTimer: Phaser.Time.TimerEvent | null = null;
  private debugGraphics!: Phaser.GameObjects.Graphics;

  private touchControls?: TouchControls;
  private menuButton!: Phaser.GameObjects.Text;
  private debugButton: Phaser.GameObjects.Text | null = null;

  constructor() {
    super("GameScene");
  }

  init(data: { mode?: "normal" | "demo" }) {
    this.mode = data?.mode ?? "normal";
  }

  create() {
    const kb = this.input.keyboard;
    if (!kb) return;

    const isCoarsePointer =
      typeof window !== "undefined" &&
      window.matchMedia("(pointer: coarse)").matches;

    const w = this.scale.width;
    const h = this.scale.height;
    const isPortrait = h > w;

    this.layout = computeLayout(w, h, {
      cols: isPortrait ? COLS_PORTRAIT : COLS,
      rows: isPortrait ? ROWS_PORTRAIT : ROWS,
      enableDPad: this.mode === "normal" && isCoarsePointer && !isPortrait,
      hasDebugButton: this.mode === "demo",
    });

    this.graphics = this.add.graphics();
    this.debugGraphics = this.add.graphics().setDepth(15);

    // Dedicated GameObjects for juice: food (depth 5), head (depth 10)
    const L = this.layout;
    this.foodCircle = this.add
      .circle(0, 0, L.cell / 2 - 2, FOOD_COLOR)
      .setOrigin(0.5)
      .setVisible(false)
      .setDepth(5);
    this.headRect = this.add
      .rectangle(0, 0, L.cell - 1, L.cell - 1, SNAKE_HEAD_COLOR)
      .setOrigin(0.5)
      .setVisible(false)
      .setDepth(10);

    // Head glow — slightly larger low-alpha rectangle behind the head
    this.headGlow = this.add
      .rectangle(0, 0, L.cell + 6, L.cell + 6, SNAKE_HEAD_COLOR, 0.18)
      .setOrigin(0.5)
      .setVisible(false)
      .setDepth(9);

    // Eyes — two small dots that sit on the head
    const eyeRadius = Math.max(1.5, L.cell * 0.12);
    this.eyeLeft = this.add
      .circle(0, 0, eyeRadius, SNAKE_EYE_COLOR)
      .setOrigin(0.5)
      .setVisible(false)
      .setDepth(11);
    this.eyeRight = this.add
      .circle(0, 0, eyeRadius, SNAKE_EYE_COLOR)
      .setOrigin(0.5)
      .setVisible(false)
      .setDepth(11);

    // One-time particle texture
    if (!this.textures.exists("particle-food")) {
      const pg = this.add.graphics();
      pg.fillStyle(FOOD_COLOR, 1);
      pg.fillCircle(4, 4, 4);
      pg.generateTexture("particle-food", 8, 8);
      pg.destroy();
    }

    if (this.mode === "normal") {
      this.cursors = kb.createCursorKeys();
    }
    this.engine = new SnakeGame({ cols: COLS, rows: ROWS });

    // Menu button (top-left, always visible)
    this.menuButton = this.add
      .text(L.menuBtnX, L.menuBtnY, "☰", {
        fontFamily: "monospace",
        fontSize: "20px",
        fontStyle: "bold",
        color: "#00cec9",
      })
      .setOrigin(0, 0.5)
      .setDepth(20)
      .setInteractive({
        hitArea: new Phaser.Geom.Rectangle(-12, -22, 44, 44),
        hitAreaCallback: Phaser.Geom.Rectangle.Contains,
        useHandCursor: true,
      });
    this.menuButton.on("pointerup", () => this.scene.start("MenuScene"));
    if (kb) {
      kb.on("keydown-M", () => this.scene.start("MenuScene"));
    }

    // Score (top area, right of menu button) — the hero stat
    this.scoreText = this.add
      .text(L.scoreX, L.scoreY, "Score: 0", {
        fontFamily: "monospace",
        fontSize: "18px",
        color: "#ffeaa7",
        backgroundColor: "#2d3436",
        padding: { x: 10, y: 4 },
      })
      .setDepth(20);

    let savedBest: number | null = null;
    try {
      const raw = localStorage.getItem("snakey-best-score");
      if (raw !== null) {
        const parsed = parseInt(raw, 10);
        if (!isNaN(parsed)) savedBest = parsed;
      }
    } catch {
      // localStorage unavailable
    }
    this.bestScore = savedBest ?? 0;
    this.bestScoreText = this.add
      .text(
        L.bestX,
        L.bestY,
        savedBest !== null ? `Best: ${savedBest}` : "Best: —",
        {
          fontFamily: "monospace",
          fontSize: "18px",
          color: "#e0e0e0",
          backgroundColor: "#2d3436",
          padding: { x: 10, y: 4 },
        },
      )
      .setOrigin(1, 0)
      .setDepth(20);

    this.tierBadgePill = this.add.graphics();
    this.tierBadgeText = this.add
      .text(0, 0, "", {
        fontFamily: "monospace",
        fontSize: "14px",
        color: "#00cec9",
      })
      .setOrigin(0.5);
    this.tierBadge = this.add
      .container(L.gameOverX, L.headerH + 4, [
        this.tierBadgePill,
        this.tierBadgeText,
      ])
      .setAlpha(0)
      .setVisible(false)
      .setDepth(20);

    this.gameOverText = this.add
      .text(L.gameOverX, L.gameOverY, "Game Over\nTap to restart", {
        fontFamily: "monospace",
        fontSize: "24px",
        color: "#ff6b6b",
        align: "center",
      })
      .setOrigin(0.5)
      .setVisible(false)
      .setDepth(20);

    // Debug toggle button (demo mode only)
    if (this.mode === "demo") {
      this.debugButton = this.add
        .text(L.debugBtnX, L.debugBtnY, "AI Vision: off", {
          fontFamily: "monospace",
          fontSize: "14px",
          color: "#b2bec3",
        })
        .setOrigin(1, 0.5)
        .setDepth(20)
        .setInteractive({
          hitArea: new Phaser.Geom.Rectangle(-50, -22, 100, 44),
          hitAreaCallback: Phaser.Geom.Rectangle.Contains,
          useHandCursor: true,
        });
      this.debugButton.on("pointerup", () => this.toggleDebug());
      if (kb) {
        kb.on("keydown-V", () => this.toggleDebug());
      }
    }

    // Touch controls (tap + swipe + optional D-pad)
    this.touchControls = new TouchControls(this, {
      onSwipe: this.mode === "normal" ? (dir: Point) => this.enqueue(dir) : null,
      onTap: () => this.restartIfDead(),
      enableDPad: this.mode === "normal" && isCoarsePointer && !isPortrait,
      initialLayout: this.layout,
    });

    // Space to restart (keyboard fallback)
    kb.on("keydown-SPACE", () => this.restartIfDead());

    this.scale.on("resize", this.handleResize, this);

    this.startGame();
  }

  private startGame() {
    this.engine.start();
    this.directionQueue = [];
    this.moveTimer = 0;
    this.scoreText.setText("Score: 0");
    this.lastDisplayedScore = 0;
    this.tweens.killTweensOf(this.scoreText);
    this.scoreText.setScale(1);
    this.tweens.killTweensOf(this.gameOverText);
    this.gameOverText.setAlpha(1).setScale(1).setVisible(false);
    this.aiDebug = null;
    this.showAIDebug = false;
    this.tierBadge?.setVisible(false);
    this.debugGraphics?.clear();
    this.debugButton?.setText("AI Vision: off");
    this.debugButton?.setColor("#b2bec3");
    // Reset juice state
    this.tweens.killTweensOf([this.headRect, this.headGlow, this.eyeLeft, this.eyeRight, this.foodCircle]);
    this.headRect?.setScale(1).setVisible(false);
    this.headGlow?.setVisible(false);
    this.eyeLeft?.setVisible(false);
    this.eyeRight?.setVisible(false);
    this.foodCircle?.setScale(1).setVisible(false);
    this.lastFoodKey = null;
    this.draw();
  }

  private restartIfDead() {
    if (!this.engine.getState().alive) this.startGame();
  }

  update(_time: number, delta: number) {
    const state = this.engine.getState();
    if (!state.alive) return;

    if (this.mode === "normal") {
      this.handleInput();
    }

    this.moveTimer += delta;
    if (this.moveTimer >= this.moveInterval) {
      this.moveTimer = 0;
      let dir: Point;
      if (this.mode === "demo") {
        const ai = getAIDirection(state, COLS, ROWS);
        this.aiDebug = ai.debug;
        dir = ai.dir;
      } else {
        dir = this.directionQueue.shift() ?? state.direction;
      }
      const result = this.engine.step(dir);
      const newState = this.engine.getState();
      this.scoreText.setText(`Score: ${newState.score}`);

      if (newState.score > this.lastDisplayedScore) {
        this.lastDisplayedScore = newState.score;
        this.pulseScore();
      }

      if (result.ate) {
        this.spawnEatParticles(newState.snake[0]);
        this.pulseHead();
      }

      if (result.won) {
        this.win();
      } else if (result.died) {
        this.die();
      } else {
        this.draw();
      }
    }
  }

  private handleInput() {
    const state = this.engine.getState();
    const refDir =
      this.directionQueue.length > 0
        ? this.directionQueue[this.directionQueue.length - 1]
        : state.direction;

    if (this.cursors.left.isDown && refDir.x !== 1) {
      this.enqueue({ x: -1, y: 0 });
    } else if (this.cursors.right.isDown && refDir.x !== -1) {
      this.enqueue({ x: 1, y: 0 });
    } else if (this.cursors.up.isDown && refDir.y !== 1) {
      this.enqueue({ x: 0, y: -1 });
    } else if (this.cursors.down.isDown && refDir.y !== -1) {
      this.enqueue({ x: 0, y: 1 });
    }
  }

  private enqueue(dir: Point) {
    if (this.directionQueue.length >= 2) return;
    const last =
      this.directionQueue.length > 0
        ? this.directionQueue[this.directionQueue.length - 1]
        : this.engine.getState().direction;
    if (dir.x === -last.x && dir.y === -last.y) return;
    this.directionQueue.push(dir);
  }

  private die() {
    this.persistBestScore();
    this.gameOverText.setText("Game Over\nTap to retry");
    this.revealGameOver();
    this.tierBadge?.setVisible(false);
    this.cameras.main.shake(200, 0.005);
    this.draw();
  }

  private win() {
    this.persistBestScore();
    this.gameOverText.setText("You Win!\nTap to retry");
    this.revealGameOver();
    this.tierBadge?.setVisible(false);
    this.draw();
  }

  private revealGameOver() {
    this.tweens.killTweensOf(this.gameOverText);
    this.gameOverText.setAlpha(0).setScale(0.9).setVisible(true);
    this.tweens.add({
      targets: this.gameOverText,
      alpha: 1,
      scaleX: 1,
      scaleY: 1,
      duration: 250,
      ease: "Back.easeOut",
    });
  }

  private persistBestScore() {
    const score = this.engine.getState().score;
    if (score > this.bestScore) {
      this.bestScore = score;
      this.bestScoreText.setText(`Best: ${this.bestScore}`);
      try {
        localStorage.setItem("snakey-best-score", String(this.bestScore));
      } catch {
        // localStorage unavailable
      }
    }
  }

  /* ──────────── Responsive layout ──────────── */

  private handleResize(gameSize: Phaser.Structs.Size) {
    const w = gameSize.width;
    const h = gameSize.height;
    this.cameras.main.setSize(w, h);

    const isCoarsePointer =
      typeof window !== "undefined" &&
      window.matchMedia("(pointer: coarse)").matches;

    const isPortrait = h > w;

    this.layout = computeLayout(w, h, {
      cols: isPortrait ? COLS_PORTRAIT : COLS,
      rows: isPortrait ? ROWS_PORTRAIT : ROWS,
      enableDPad: this.mode === "normal" && isCoarsePointer && !isPortrait,
      hasDebugButton: this.mode === "demo",
    });

    // Reposition HUD
    this.scoreText.setPosition(this.layout.scoreX, this.layout.scoreY);
    this.bestScoreText.setPosition(this.layout.bestX, this.layout.bestY);
    this.gameOverText.setPosition(
      this.layout.gameOverX,
      this.layout.gameOverY,
    );
    this.tierBadge.setPosition(this.layout.gameOverX, this.layout.headerH + 4);
    this.menuButton.setPosition(this.layout.menuBtnX, this.layout.menuBtnY);
    this.debugButton?.setPosition(
      this.layout.debugBtnX,
      this.layout.debugBtnY,
    );

    // Touch controls reposition D-pad + deadzone
    this.touchControls?.relayout(this.layout);

    // Redraw with new geometry
    this.draw();
  }

  shutdown() {
    this.scale.off("resize", this.handleResize, this);
    this.touchControls?.destroy();
    if (this.tierToastTimer) {
      this.tierToastTimer.remove();
      this.tierToastTimer = null;
    }
  }

  /* ──────────── Rendering ──────────── */

  private draw() {
    const state = this.engine.getState();
    const L = this.layout;

    // Sync dynamic sizes in case cell changed on resize
    this.headRect.setSize(L.cell - 1, L.cell - 1);
    this.foodCircle.setRadius(Math.max(2, L.cell / 2 - 2));

    this.graphics.clear();
    this.drawBorder();
    this.drawGrid();

    const alpha = state.alive ? 1 : 0.4;
    this.drawBody(state.snake, alpha);
    this.drawHead(state.snake[0], alpha);
    this.drawFood(state.food, alpha);

    if (!state.alive) {
      this.headGlow.setVisible(false);
      this.eyeLeft.setVisible(false);
      this.eyeRight.setVisible(false);
    }

    if (this.mode === "demo" && this.showAIDebug && this.aiDebug) {
      this.drawAIDebug();
    }
  }

  private drawBorder() {
    const g = this.graphics;
    const L = this.layout;
    g.lineStyle(2, WALL_COLOR, 1);
    g.strokeRect(L.offsetX - 2, L.offsetY - 2, L.gridW + 4, L.gridH + 4);
  }

  private drawGrid() {
    const g = this.graphics;
    const L = this.layout;
    g.lineStyle(1, GRID_COLOR, 0.55);
    for (let x = 0; x <= L.cols; x++) {
      const px = this.gridX(x);
      g.lineBetween(px, L.offsetY, px, L.offsetY + L.gridH);
    }
    for (let y = 0; y <= L.rows; y++) {
      const py = this.gridY(y);
      g.lineBetween(L.offsetX, py, L.offsetX + L.gridW, py);
    }
  }

  private gridX(col: number) {
    return this.layout.offsetX + col * this.layout.cell;
  }

  private gridY(row: number) {
    return this.layout.offsetY + row * this.layout.cell;
  }

  private drawBody(snake: Point[], alpha: number) {
    const g = this.graphics;
    const L = this.layout;
    for (let i = 1; i < snake.length; i++) {
      const seg = snake[i];
      g.fillStyle(SNAKE_BODY_COLOR, alpha);
      g.fillRect(
        this.gridX(seg.x) + 1,
        this.gridY(seg.y) + 1,
        L.cell - 2,
        L.cell - 2,
      );
    }
  }

  private drawHead(head: Point, alpha: number) {
    const L = this.layout;
    const state = this.engine.getState();
    const dir = state.direction;

    const cx = this.gridX(head.x) + L.cell / 2;
    const cy = this.gridY(head.y) + L.cell / 2;

    // Head glow — slightly larger, low-alpha rectangle behind the head
    this.headGlow
      .setPosition(cx, cy)
      .setFillStyle(SNAKE_HEAD_COLOR, alpha * 0.18)
      .setVisible(true);

    // Head rect — 1px larger than body cells
    this.headRect
      .setPosition(cx, cy)
      .setSize(L.cell - 1, L.cell - 1)
      .setFillStyle(SNAKE_HEAD_COLOR, alpha)
      .setVisible(true);

    // Eyes positioned based on travel direction
    const eyeOffset = L.cell * 0.22;
    let ex1: number, ey1: number, ex2: number, ey2: number;

    if (dir.x === 0 && dir.y === -1) {
      // Up — eyes toward the top
      ex1 = cx - eyeOffset; ey1 = cy - eyeOffset;
      ex2 = cx + eyeOffset; ey2 = cy - eyeOffset;
    } else if (dir.x === 0 && dir.y === 1) {
      // Down — eyes toward the bottom
      ex1 = cx - eyeOffset; ey1 = cy + eyeOffset;
      ex2 = cx + eyeOffset; ey2 = cy + eyeOffset;
    } else if (dir.x === -1 && dir.y === 0) {
      // Left — eyes toward the left
      ex1 = cx - eyeOffset; ey1 = cy - eyeOffset;
      ex2 = cx - eyeOffset; ey2 = cy + eyeOffset;
    } else {
      // Right (dir.x === 1 && dir.y === 0) — eyes toward the right
      ex1 = cx + eyeOffset; ey1 = cy - eyeOffset;
      ex2 = cx + eyeOffset; ey2 = cy + eyeOffset;
    }

    this.eyeLeft
      .setPosition(ex1, ey1)
      .setFillStyle(SNAKE_EYE_COLOR, alpha)
      .setVisible(true);
    this.eyeRight
      .setPosition(ex2, ey2)
      .setFillStyle(SNAKE_EYE_COLOR, alpha)
      .setVisible(true);
  }

  private drawFood(foodPos: Point | null, alpha: number) {
    if (!foodPos) {
      this.foodCircle.setVisible(false);
      this.lastFoodKey = null;
      return;
    }
    const L = this.layout;
    const k = `${foodPos.x},${foodPos.y}`;
    this.foodCircle
      .setPosition(
        this.gridX(foodPos.x) + L.cell / 2,
        this.gridY(foodPos.y) + L.cell / 2,
      )
      .setFillStyle(FOOD_COLOR, alpha)
      .setVisible(true);
    if (k !== this.lastFoodKey) {
      this.lastFoodKey = k;
      this.tweens.killTweensOf(this.foodCircle);
      this.foodCircle.setScale(0);
      this.tweens.add({
        targets: this.foodCircle,
        scaleX: 1,
        scaleY: 1,
        duration: 200,
        ease: "Back.easeOut",
      });
    }
  }

  /* ──────────── Juice ──────────── */

  private spawnEatParticles(head: Point) {
    const L = this.layout;
    const x = this.gridX(head.x) + L.cell / 2;
    const y = this.gridY(head.y) + L.cell / 2;
    const emitter = this.add.particles(x, y, "particle-food", {
      speed: { min: 40, max: 140 },
      angle: { min: 0, max: 360 },
      lifespan: { min: 300, max: 600 },
      quantity: 1,
      scale: { start: 1, end: 0 },
      alpha: { start: 1, end: 0 },
      blendMode: "ADD",
      emitting: false,
    });
    emitter.setDepth(12);
    emitter.explode(20);
    this.time.delayedCall(700, () => emitter.destroy());
  }

  private pulseHead() {
    this.tweens.killTweensOf(this.headRect);
    this.headRect.setScale(1);
    this.tweens.add({
      targets: this.headRect,
      scaleX: 1.15,
      scaleY: 1.15,
      duration: 80,
      yoyo: true,
      ease: "Quad.easeOut",
    });
  }

  private pulseScore() {
    const t = this.scoreText;
    this.tweens.killTweensOf(t);
    t.setScale(1);
    this.tweens.add({
      targets: t,
      scaleX: 1.18,
      scaleY: 1.18,
      duration: 120,
      yoyo: true,
      ease: "Quad.easeOut",
    });
  }

  /* ──────────── AI Debug Visualization ──────────── */

  private toggleDebug() {
    this.showAIDebug = !this.showAIDebug;
    this.debugButton?.setText(
      `AI Vision: ${this.showAIDebug ? "on" : "off"}`,
    );
    this.debugButton?.setColor(this.showAIDebug ? "#00cec9" : "#b2bec3");
    if (!this.showAIDebug) {
      this.lastTier = null;
      if (this.tierToastTimer) {
        this.tierToastTimer.remove();
        this.tierToastTimer = null;
      }
      this.tierBadge.setVisible(false);
      this.tierBadge.setAlpha(0);
      this.debugGraphics.clear();
    }
  }

  private showTierToast(tier: string, color: string, label: string) {
    if (tier === this.lastTier) return;
    this.lastTier = tier;

    this.tierBadgeText.setText(label);
    this.tierBadgeText.setColor(color);
    this.tierBadge.setAlpha(0);
    this.tierBadge.setVisible(true);

    this.tweens.killTweensOf(this.tierBadge);

    this.tweens.add({
      targets: this.tierBadge,
      alpha: 1,
      duration: 200,
      ease: "Power2",
    });

    if (this.tierToastTimer) {
      this.tierToastTimer.remove();
      this.tierToastTimer = null;
    }

    this.tierToastTimer = this.time.delayedCall(2500, () => {
      this.tweens.add({
        targets: this.tierBadge,
        alpha: 0,
        duration: 400,
        ease: "Power2",
        onComplete: () => {
          this.tierBadge.setVisible(false);
        },
      });
    });
  }

  private drawAIDebug() {
    if (!this.aiDebug) return;
    const g = this.debugGraphics;
    g.clear();

    // Flood-fill region
    g.fillStyle(0x6c5ce7, 0.15);
    for (const cell of this.aiDebug.reachable) {
      g.fillRect(
        this.gridX(cell.x) + 1,
        this.gridY(cell.y) + 1,
        this.layout.cell - 2,
        this.layout.cell - 2,
      );
    }

    // BFS path (dashed)
    const path = this.aiDebug.path;
    if (path.length >= 2) {
      g.lineStyle(2, 0xff7675, 0.9);
      for (let i = 1; i < path.length; i++) {
        const from = path[i - 1];
        const to = path[i];
        const x1 = this.gridX(from.x) + this.layout.cell / 2;
        const y1 = this.gridY(from.y) + this.layout.cell / 2;
        const x2 = this.gridX(to.x) + this.layout.cell / 2;
        const y2 = this.gridY(to.y) + this.layout.cell / 2;
        const dx = x2 - x1;
        const dy = y2 - y1;
        const dist = Math.sqrt(dx * dx + dy * dy);
        const steps = Math.floor(dist / 6);
        if (steps < 1) {
          g.lineBetween(x1, y1, x2, y2);
          continue;
        }
        const stepX = dx / steps;
        const stepY = dy / steps;
        for (let s = 0; s < steps; s++) {
          if (s % 2 === 0) {
            g.lineBetween(
              x1 + s * stepX,
              y1 + s * stepY,
              x1 + (s + 1) * stepX,
              y1 + (s + 1) * stepY,
            );
          }
        }
      }
    }

    // Tier badge
    const tier = this.aiDebug.tier;
    const colors: Record<string, string> = {
      tier1: "#00cec9",
      tier2: "#fdcb6e",
      fallback: "#ff7675",
    };
    const labels: Record<string, string> = {
      tier1: "Tier 1 \u00B7 Safe Pursuit",
      tier2: "Tier 2 \u00B7 Max Space",
      fallback: "Fallback \u00B7 Toward Food",
    };
    this.showTierToast(tier, colors[tier], labels[tier]);
  }
}
