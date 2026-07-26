import Phaser from "phaser";
import { BACKGROUND_COLOR, FOOD_COLOR } from "../config";

/* ──────────── Constants ──────────── */

const CARD_W = 260;
const CARD_H = 160;
const CARD_RADIUS = 14;
const CARD_GAP = 30;

const COLOR_CYAN = 0x00cec9;
const COLOR_PURPLE = 0x6c5ce7;

const MUTED_TEXT = "#636e72";
const BRIGHT_MUTED = "#b2bec3";
const ACCENT_CYAN_STR = "#00cec9";
const ACCENT_PURPLE_STR = "#6c5ce7";

/* ================================================================
   MENU SCENE
   ================================================================ */

export class MenuScene extends Phaser.Scene {
  private cardsReady = false;

  constructor() {
    super({ key: "MenuScene" });
  }

  /* ────── Preload ────── */

  preload() {
    this.load.json("benchmark", "benchmark.json");
  }

  /* ────── Create ────── */

  create() {
    const W = this.scale.width;
    const H = this.scale.height;
    const cx = W / 2;

    this.cameras.main.setBackgroundColor(BACKGROUND_COLOR);
    this.drawBackgroundDecoration(W, H);

    /* ---- Card layout: landscape = side-by-side, portrait = stacked ---- */
    const isPortrait = H > W;
    const cardsCenterY = H * 0.52;

    let leftCardX: number;
    let rightCardX: number;
    let leftCardY: number;
    let rightCardY: number;

    if (isPortrait) {
      leftCardX = rightCardX = cx;
      leftCardY = cardsCenterY - CARD_H / 2 - CARD_GAP / 2;
      rightCardY = cardsCenterY + CARD_H / 2 + CARD_GAP / 2;
    } else {
      leftCardX = cx - CARD_W / 2 - CARD_GAP / 2;
      rightCardX = cx + CARD_W / 2 + CARD_GAP / 2;
      leftCardY = rightCardY = cardsCenterY;
    }

    /* ---- Title ---- */
    const title = this.add
      .text(cx, H * 0.14, "SNAKEY", {
        fontFamily: "monospace",
        fontSize: Math.min(68, W * 0.12) + "px",
        color: ACCENT_CYAN_STR,
        fontStyle: "bold",
      })
      .setOrigin(0.5)
      .setAlpha(0);

    /* ---- Subtitle ---- */
    const subtitle = this.add
      .text(cx, H * 0.14 + 60, "Choose your mode", {
        fontFamily: "monospace",
        fontSize: Math.min(18, W * 0.035) + "px",
        color: MUTED_TEXT,
      })
      .setOrigin(0.5)
      .setAlpha(0);

    /* ---- Cards ---- */
    const normalCard = this.createModeCard(
      leftCardX,
      leftCardY,
      "NORMAL",
      "Tap or use arrow keys",
      COLOR_CYAN,
      ACCENT_CYAN_STR,
      () => this.scene.start("GameScene", { mode: "normal" }),
    );
    normalCard.setAlpha(0).setScale(0.85);

    const demoCard = this.createModeCard(
      rightCardX,
      rightCardY,
      "DEMO",
      "Watch the AI",
      COLOR_PURPLE,
      ACCENT_PURPLE_STR,
      () => this.scene.start("GameScene", { mode: "demo" }),
    );
    demoCard.setAlpha(0).setScale(0.85);

    /* ---- Best Score ---- */
    let bestScoreLabel = "Best: \u2014";
    try {
      const raw = localStorage.getItem("snakey-best-score");
      if (raw !== null) {
        const n = parseInt(raw, 10);
        if (!isNaN(n)) bestScoreLabel = `Best: ${n}`;
      }
    } catch {
      /* localStorage unavailable */
    }

    const scoreText = this.add
      .text(cx, H - 62, bestScoreLabel, {
        fontFamily: "monospace",
        fontSize: "16px",
        color: MUTED_TEXT,
      })
      .setOrigin(0.5)
      .setAlpha(0);

    /* ---- Keyboard shortcuts ---- */
    const kb = this.input.keyboard;
    if (kb) {
      kb.on("keydown-N", () =>
        this.scene.start("GameScene", { mode: "normal" }),
      );
      kb.on("keydown-D", () =>
        this.scene.start("GameScene", { mode: "demo" }),
      );
    }

    /* ---- Orchestrated entrance ---- */
    this.tweens.add({
      targets: title,
      alpha: 1,
      y: H * 0.14,
      duration: 600,
      ease: "Power3.easeOut",
    });

    this.tweens.add({
      targets: subtitle,
      alpha: 1,
      duration: 400,
      delay: 180,
      ease: "Power2.easeOut",
    });

    this.tweens.add({
      targets: normalCard,
      alpha: 1,
      scaleX: 1,
      scaleY: 1,
      duration: 450,
      delay: 350,
      ease: "Back.easeOut",
    });

    this.tweens.add({
      targets: demoCard,
      alpha: 1,
      scaleX: 1,
      scaleY: 1,
      duration: 450,
      delay: 500,
      ease: "Back.easeOut",
      onComplete: () => {
        this.cardsReady = true;
      },
    });

    this.tweens.add({
      targets: scoreText,
      alpha: 1,
      duration: 400,
      delay: 700,
      ease: "Power2.easeOut",
    });

    /* ---- AI Benchmark Stats ---- */
    const bm = this.cache.json.get("benchmark") as
      | { runs: number; version: number; max: number; winRate: number }
      | undefined;
    const aiStatsLabel =
      bm && bm.runs > 0
        ? `AI best: ${bm.max} \u00B7 win rate: ${(bm.winRate * 100).toFixed(0)}%`
        : "AI: run `npm run benchmark`";

    const aiStatsText = this.add
      .text(cx, H - 34, aiStatsLabel, {
        fontFamily: "monospace",
        fontSize: "14px",
        color: ACCENT_PURPLE_STR,
      })
      .setOrigin(0.5)
      .setAlpha(0);

    this.tweens.add({
      targets: aiStatsText,
      alpha: 1,
      duration: 400,
      delay: 850,
      ease: "Power2.easeOut",
    });

    /* ---- Resize listener: restart to re-layout ---- */
    // Use scene events (not shutdown override) for guaranteed cleanup on scene.stop()
    this.events.on("shutdown", () => {
      this.scale.off("resize", this.onResize, this);
    });
    this.scale.on("resize", this.onResize, this);
  }

  private onResize() {
    this.scale.off("resize", this.onResize, this);
    this.scene.restart();
  }

  /* ────── Background Decoration ────── */

  private drawBackgroundDecoration(w: number, h: number) {
    const g = this.add.graphics();
    const margin = Math.floor(w * 0.03);
    const gridStep = Math.floor(w / 30);

    /* Subtle grid */
    g.lineStyle(1, 0xffffff, 0.025);
    for (let x = 0; x <= w; x += gridStep) {
      g.lineBetween(x, 0, x, h);
    }
    for (let y = 0; y <= h; y += gridStep) {
      g.lineBetween(0, y, w, y);
    }

    /* Faint horizontal accent lines */
    const accentAlpha = 0.06;
    g.lineStyle(2, COLOR_CYAN, accentAlpha);
    g.lineBetween(margin, 155, w / 2 - 80, 155);
    g.lineBetween(w / 2 + 80, 155, w - margin, 155);

    g.lineStyle(1, COLOR_PURPLE, accentAlpha * 0.8);
    g.lineBetween(margin, h * 0.78, w - margin, h * 0.78);

    /* Winding snake-like decorative path */
    g.lineStyle(1.5, COLOR_CYAN, 0.04);
    const startX = w - 60;
    const startY = h - 100;
    g.beginPath();
    g.moveTo(startX, startY);
    g.lineTo(startX - 40, startY - 30);
    g.lineTo(startX - 120, startY - 50);
    g.lineTo(startX - 180, startY - 20);
    g.lineTo(startX - 200, startY + 10);
    g.strokePath();

    /* Scattered subtle dots along the path */
    g.fillStyle(COLOR_CYAN, 0.06);
    const dots: [number, number][] = [
      [startX - 20, startY - 15],
      [startX - 60, startY - 38],
      [startX - 100, startY - 48],
      [startX - 150, startY - 38],
      [startX - 185, startY - 10],
      [startX - 198, startY + 5],
    ];
    for (const [dx, dy] of dots) {
      g.fillCircle(dx, dy, 3);
    }

    /* Faint food-colored accent dots */
    g.fillStyle(FOOD_COLOR, 0.05);
    g.fillCircle(margin + 30, h - 60, 4);
    g.fillCircle(w - margin - 50, h * 0.08, 3);
  }

  /* ────── Mode Card Factory ────── */

  private createModeCard(
    cx: number,
    cy: number,
    label: string,
    subtitle: string,
    accentNum: number,
    accentStr: string,
    onClick: () => void,
  ): Phaser.GameObjects.Container {
    const container = this.add.container(cx, cy);
    const hoverTween = { ref: null as Phaser.Tweens.Tween | null };

    /* ---- Card background graphics ---- */
    const gfx = this.add.graphics();
    container.add(gfx);

    /* ---- Shadow ---- */
    const shadow = this.add.graphics();
    shadow.fillStyle(0x000000, 0.25);
    shadow.fillRoundedRect(
      -CARD_W / 2 + 4,
      -CARD_H / 2 + 4,
      CARD_W,
      CARD_H,
      CARD_RADIUS,
    );
    container.addAt(shadow, 0);

    /* ---- Mode label ---- */
    const labelText = this.add
      .text(0, -18, label, {
        fontFamily: "monospace",
        fontSize: "26px",
        color: accentStr,
        fontStyle: "bold",
      })
      .setOrigin(0.5);
    container.add(labelText);

    /* ---- Subtitle ---- */
    const subText = this.add
      .text(0, 18, subtitle, {
        fontFamily: "monospace",
        fontSize: "14px",
        color: MUTED_TEXT,
      })
      .setOrigin(0.5);
    container.add(subText);

    /* ---- Initial draw ---- */
    this.drawCard(gfx, accentNum, false);

    /* ---- Interactivity ---- */
    container.setSize(CARD_W, CARD_H);
    container.setInteractive({ useHandCursor: true });

    container.on("pointerover", () => {
      if (!this.cardsReady) return;
      if (hoverTween.ref) hoverTween.ref.stop();

      this.drawCard(gfx, accentNum, true);
      labelText.setColor("#ffffff");
      subText.setColor(BRIGHT_MUTED);

      hoverTween.ref = this.tweens.add({
        targets: container,
        scaleX: 1.05,
        scaleY: 1.05,
        duration: 180,
        ease: "Back.easeOut",
      });
    });

    container.on("pointerout", () => {
      if (!this.cardsReady) return;
      if (hoverTween.ref) hoverTween.ref.stop();

      this.drawCard(gfx, accentNum, false);
      labelText.setColor(accentStr);
      subText.setColor(MUTED_TEXT);

      hoverTween.ref = this.tweens.add({
        targets: container,
        scaleX: 1,
        scaleY: 1,
        duration: 180,
        ease: "Back.easeIn",
      });
    });

    container.on("pointerdown", () => {
      this.tweens.add({
        targets: container,
        scaleX: 0.96,
        scaleY: 0.96,
        duration: 60,
        yoyo: true,
        ease: "Power2",
        onComplete: onClick,
      });
    });

    return container;
  }

  /* ────── Card Drawing ────── */

  private drawCard(
    gfx: Phaser.GameObjects.Graphics,
    accentNum: number,
    hovered: boolean,
  ) {
    gfx.clear();

    const fillAlpha = hovered ? 0.22 : 0.08;
    const strokeAlpha = hovered ? 1 : 0.55;
    const strokeWidth = hovered ? 2.5 : 2;

    /* Fill */
    gfx.fillStyle(accentNum, fillAlpha);
    gfx.fillRoundedRect(
      -CARD_W / 2,
      -CARD_H / 2,
      CARD_W,
      CARD_H,
      CARD_RADIUS,
    );

    /* Border with subtle inner glow on hover */
    if (hovered) {
      gfx.lineStyle(strokeWidth + 1, accentNum, 0.3);
      gfx.strokeRoundedRect(
        -CARD_W / 2 - 1,
        -CARD_H / 2 - 1,
        CARD_W + 2,
        CARD_H + 2,
        CARD_RADIUS + 1,
      );
    }

    /* Main border */
    gfx.lineStyle(strokeWidth, accentNum, strokeAlpha);
    gfx.strokeRoundedRect(
      -CARD_W / 2,
      -CARD_H / 2,
      CARD_W,
      CARD_H,
      CARD_RADIUS,
    );

    /* Top accent line inside card */
    gfx.lineStyle(1, accentNum, hovered ? 0.5 : 0.2);
    gfx.lineBetween(
      -CARD_W / 2 + CARD_RADIUS,
      -CARD_H / 2 + 6,
      CARD_W / 2 - CARD_RADIUS,
      -CARD_H / 2 + 6,
    );
  }
}
