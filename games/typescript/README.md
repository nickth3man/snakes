# Snake &middot; TypeScript

A Snake game built with Phaser 3 and TypeScript, bundled with Vite. The most elaborate of the six:
menu and game scenes, a self-playing demo mode, touch controls, a local leaderboard and a headless
benchmark harness, over a game engine module with its own unit tests.

## Controls

- **Arrow keys** — move the snake (Up / Down / Left / Right)
- **N** / **D** — start a normal or demo game from the menu
- **Space** or **click** — restart after game over

## Requirements

- Node.js 20+

## Getting started

```bash
npm install        # install dependencies
npm run dev        # start the dev server with HMR
npm run build      # type-check and build for production
npm test           # run the engine unit tests
npm run preview    # serve the production build locally
```

Use `npm run dev` rather than serving `dist` directly: the production build is compiled for the
`/snakes/typescript/` path it is deployed under.

For the deployed build, `build.sh` wraps `npm ci && npm run build` and stages `dist` wherever the
site build asks for it.

## Project layout

| File | Purpose |
|---|---|
| `src/main.ts` | Phaser game bootstrap and configuration |
| `src/game/engine.ts` | Framework-free rules, unit tested |
| `src/scenes/GameScene.ts` | Gameplay scene and rendering |
| `src/scenes/MenuScene.ts` | Menu, mode selection and stats |
| `src/layout.ts` | Pure responsive layout calculator |
| `src/ai/demo-controller.ts` | The snake that plays itself in demo mode |

## License

MIT — see [LICENSE](../../LICENSE) for details.
