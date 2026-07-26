import Phaser from "phaser";
import { getLeaderboard } from "./leaderboard-store";
import { formatRow } from "./leaderboard-core";

/**
 * Scrollable leaderboard panel (pure canvas, no DOM).
 *
 * - Geometry-masked scroll region: drag (touch/mouse) + wheel (desktop).
 * - No inertia by design — simple, deterministic, plays well with the
 *   MenuScene restart-on-resize lifecycle.
 * - Scene-scoped input listeners are auto-removed by Phaser on scene
 *   shutdown (MenuScene restarts on resize → panel is rebuilt).
 */

export const ROW_H = 28;
export const MIN_PANEL_H = ROW_H * 2;

const HEADER_H = 28;
const FONT_SIZE_WIDE = 14;
const FONT_SIZE_NARROW = 12;
const TEXT_COLOR = "#b2bec3";
const PLAYER_COLOR = "#00cec9";

export class LeaderboardPanel extends Phaser.GameObjects.Container {
  private readonly scroll: Phaser.GameObjects.Container;
  private scrollY = 0;
  private readonly maxScroll: number;
  private readonly panelX: number;
  private readonly panelW: number;
  private readonly maskY: number;
  private readonly maskH: number;
  private dragging = false;
  private dragStartY = 0;
  private dragStartScroll = 0;

  constructor(scene: Phaser.Scene, x: number, y: number, w: number, h: number) {
    super(scene, 0, 0);
    this.panelX = x;
    this.panelW = w;
    this.maskY = y + HEADER_H;
    this.maskH = h - HEADER_H;
    scene.add.existing(this);

    /* ---- Chrome (unmasked) ---- */
    const bg = scene.add.graphics();
    bg.fillStyle(0x0f0f1e, 0.6);
    bg.fillRoundedRect(x, y, w, h, 8);
    bg.lineStyle(1, 0x00cec9, 0.35);
    bg.strokeRoundedRect(x, y, w, h, 8);
    bg.lineStyle(1, 0x6c5ce7, 0.5);
    bg.lineBetween(x + 12, y, x + w - 12, y);
    this.add(bg);

    const header = scene.add
      .text(x + w / 2, y + HEADER_H / 2, "\u2500\u2500 LEADERBOARD \u2500\u2500", {
        fontFamily: "monospace",
        fontSize: "12px",
        color: "#6c5ce7",
      })
      .setOrigin(0.5);
    this.add(header);

    /* ---- Masked scroll region ---- */
    const maskShape = scene.make.graphics({ add: false } as Parameters<typeof scene.make.graphics>[0] & { add: boolean });
    maskShape.fillStyle(0xffffff);
    maskShape.fillRect(x, this.maskY, w, this.maskH);
    this.scroll = scene.add.container(0, this.maskY);
    this.scroll.setMask(maskShape.createGeometryMask());
    this.add(this.scroll);

    // Narrow panels get a smaller font so rows (up to ~40 chars) still fit.
    const fontSize = w < 340 ? FONT_SIZE_NARROW : FONT_SIZE_WIDE;

    const rows = getLeaderboard();
    rows.forEach((row, i) => {
      const t = scene.add.text(x + 10, i * ROW_H, formatRow(row), {
        fontFamily: "monospace",
        fontSize: `${fontSize}px`,
        color: row.isPlayer ? PLAYER_COLOR : TEXT_COLOR,
      });
      this.scroll.add(t);
    });
    this.maxScroll = Math.max(0, rows.length * ROW_H - this.maskH);

    /* ---- Input: drag surface (touch + mouse) ---- */
    const hit = scene.add.rectangle(
      x + w / 2,
      this.maskY + this.maskH / 2,
      w,
      this.maskH,
      0x000000,
      0.001,
    );
    hit.setInteractive();
    hit.on("pointerdown", (p: Phaser.Input.Pointer) => {
      this.dragging = true;
      this.dragStartY = p.y;
      this.dragStartScroll = this.scrollY;
    });
    this.add(hit);

    scene.input.on("pointermove", this.handleMove, this);
    scene.input.on("pointerup", this.handleUp, this);
    scene.input.on("pointerupoutside", this.handleUp, this);
    scene.input.on("wheel", this.handleWheel, this);
  }

  private handleMove(p: Phaser.Input.Pointer) {
    if (!this.dragging) return;
    this.scrollTo(this.dragStartScroll - (p.y - this.dragStartY));
  }

  private handleUp() {
    this.dragging = false;
  }

  private handleWheel(
    pointer: Phaser.Input.Pointer,
    _gameObjects: Phaser.GameObjects.GameObject[],
    _dx: number,
    dy: number,
  ) {
    const inside =
      pointer.x >= this.panelX &&
      pointer.x <= this.panelX + this.panelW &&
      pointer.y >= this.maskY &&
      pointer.y <= this.maskY + this.maskH;
    if (inside) this.scrollTo(this.scrollY + dy);
  }

  private scrollTo(v: number) {
    this.scrollY = Phaser.Math.Clamp(v, 0, this.maxScroll);
    this.scroll.y = this.maskY - this.scrollY;
  }
}
