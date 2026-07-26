import Phaser from "phaser";
import type { Point } from "../game/engine";
import type { Layout } from "../layout";

/**
 * Touch / pointer controls for Snakey.
 *
 * Under Scale.RESIZE the pointer coordinates are raw canvas pixels.
 * Swipe detection is relative (deltas), so it is coordinate-system invariant;
 * only the deadzone threshold and the D-pad suppression rectangle depend on
 * absolute geometry — both are derived from the current Layout.
 */

export interface TouchControlsOptions {
  /** Called with a swipe direction. `null` in demo mode (no swipe / no D-pad). */
  onSwipe: ((dir: Point) => void) | null;
  /** Called on a clean tap. Used for restart-on-death in both modes. */
  onTap: () => void;
  /** When true, an on-screen D-pad is created (pass true only for touch devices). */
  enableDPad: boolean;
  /** Layout snapshot at creation time. */
  initialLayout: Layout;
}

interface Gesture {
  startX: number;
  startY: number;
  startTime: number;
  /** True once a swipe direction has been emitted for this gesture. */
  fired: boolean;
  /** True if the gesture began on the D-pad (field-swipe suppressed). */
  suppressed: boolean;
}

const TAP_MAX_TIME = 250; // ms; slower than this is not a tap

export class TouchControls {
  private readonly scene: Phaser.Scene;
  private readonly onSwipe: ((dir: Point) => void) | null;
  private readonly onTap: () => void;
  private layout: Layout;
  private readonly gestures = new Map<number, Gesture>();
  private dpadBounds: Phaser.Geom.Rectangle | null = null;
  private dpadButtons: Phaser.GameObjects.Text[] = [];

  constructor(scene: Phaser.Scene, opts: TouchControlsOptions) {
    this.scene = scene;
    this.onSwipe = opts.onSwipe;
    this.onTap = opts.onTap;
    this.layout = opts.initialLayout;

    scene.input.on("pointerdown", this.handleDown, this);
    scene.input.on("pointermove", this.handleMove, this);
    scene.input.on("pointerup", this.handleUp, this);
    scene.input.on("pointerupoutside", this.handleUp, this);

    if (opts.enableDPad && this.onSwipe) {
      this.createDPad();
    }
  }

  /* ──────────── Layout refresh (called by GameScene on viewport resize) ──────────── */

  relayout(layout: Layout) {
    this.layout = layout;
    // In-flight gestures have stale start positions — clear them.
    this.gestures.clear();

    if (this.dpadButtons.length === 0) return;

    const { dpadCx: cx, dpadCy: cy, dpadStep: step, dpadFontSize: fs } =
      layout;
    const fsStr = `${fs}px`;

    // Positions: [up, down, left, right] — same order as createDPad
    const positions: [number, number][] = [
      [cx, cy - step],
      [cx, cy + step],
      [cx - step, cy],
      [cx + step, cy],
    ];
    this.dpadButtons.forEach((btn, i) => {
      btn.setPosition(positions[i][0], positions[i][1]);
      btn.setStyle({ fontSize: fsStr });
    });

    this.recalcDpadBounds();
  }

  /* ──────────── Pointer state machine ──────────── */

  private deadzone(): number {
    // Always >=24 CSS px so the threshold stays usable on big screens
    return Math.max(24, this.layout.cell);
  }

  private handleDown(pointer: Phaser.Input.Pointer) {
    const suppressed =
      this.dpadBounds !== null &&
      this.dpadBounds.contains(pointer.x, pointer.y);
    this.gestures.set(pointer.id, {
      startX: pointer.x,
      startY: pointer.y,
      startTime: this.scene.time.now,
      fired: false,
      suppressed,
    });
  }

  private handleMove(pointer: Phaser.Input.Pointer) {
    if (!this.onSwipe) return;
    const g = this.gestures.get(pointer.id);
    if (!g || g.fired || g.suppressed) return;

    const dx = pointer.x - g.startX;
    const dy = pointer.y - g.startY;
    if (Math.max(Math.abs(dx), Math.abs(dy)) < this.deadzone()) return;

    g.fired = true;
    // Axis-lock: pick the dominant axis, ignore the minor one.
    const dir: Point =
      Math.abs(dx) > Math.abs(dy)
        ? { x: dx < 0 ? -1 : 1, y: 0 }
        : { x: 0, y: dy < 0 ? -1 : 1 };
    this.onSwipe(dir);
  }

  private handleUp(pointer: Phaser.Input.Pointer) {
    const g = this.gestures.get(pointer.id);
    if (!g) return;

    // A tap is a short, stationary press that did not fire a swipe and was not
    // on the D-pad. This keeps swipes from restarting and taps from steering.
    if (!g.fired && !g.suppressed) {
      const dx = pointer.x - g.startX;
      const dy = pointer.y - g.startY;
      const dist = Math.sqrt(dx * dx + dy * dy);
      const dt = this.scene.time.now - g.startTime;
      if (dist <= this.deadzone() && dt <= TAP_MAX_TIME) {
        this.onTap();
      }
    }

    this.gestures.delete(pointer.id);
  }

  /* ──────────── D-pad ──────────── */

  private createDPad() {
    const onSwipe = this.onSwipe;
    if (!onSwipe) return; // narrowed for the closure below

    const L = this.layout;

    const make = (
      x: number,
      y: number,
      glyph: string,
      dir: Point,
    ): Phaser.GameObjects.Text => {
      const btn = this.scene.add
        .text(x, y, glyph, {
          fontFamily: "monospace",
          fontSize: `${L.dpadFontSize}px`,
          color: "#00cec9",
        })
        .setOrigin(0.5)
        .setAlpha(0.25)
        .setDepth(20)
        .setInteractive(
          new Phaser.Geom.Rectangle(-22, -22, 44, 44),
          Phaser.Geom.Rectangle.Contains,
        );

      btn.on("pointerover", () => btn.setAlpha(0.7));
      btn.on("pointerout", () => btn.setAlpha(0.25));
      btn.on("pointerdown", () => {
        btn.setAlpha(0.7);
        this.scene.tweens.add({
          targets: btn,
          scaleX: 0.88,
          scaleY: 0.88,
          duration: 60,
          yoyo: true,
          ease: "Power2",
        });
        onSwipe(dir);
      });
      this.dpadButtons.push(btn);
      return btn;
    };

    make(L.dpadCx, L.dpadCy - L.dpadStep, "\u25B2", { x: 0, y: -1 }); // up
    make(L.dpadCx, L.dpadCy + L.dpadStep, "\u25BC", { x: 0, y: 1 }); // down
    make(L.dpadCx - L.dpadStep, L.dpadCy, "\u25C0", { x: -1, y: 0 }); // left
    make(L.dpadCx + L.dpadStep, L.dpadCy, "\u25B6", { x: 1, y: 0 }); // right

    this.recalcDpadBounds();
  }

  private recalcDpadBounds() {
    const { dpadCx: cx, dpadCy: cy, dpadStep: step } = this.layout;
    const pad = step + 24;
    this.dpadBounds = new Phaser.Geom.Rectangle(
      cx - pad,
      cy - pad,
      pad * 2,
      pad * 2,
    );
  }

  /* ──────────── Teardown ──────────── */

  destroy() {
    this.scene.input.off("pointerdown", this.handleDown, this);
    this.scene.input.off("pointermove", this.handleMove, this);
    this.scene.input.off("pointerup", this.handleUp, this);
    this.scene.input.off("pointerupoutside", this.handleUp, this);
    this.gestures.clear();
  }
}
