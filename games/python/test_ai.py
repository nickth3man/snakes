"""Checks the Python demo AI against decisions recorded from the reference.

``testdata/ai-trace.json`` comes from the Rust module, which ai-parity.mts
checks against the original TypeScript demo controller -- so matching the trace
means matching the original.

main.py imports the browser-only ``js`` and ``pyodide`` modules, so this stubs
them out before importing it. Run with:

    python -m unittest discover games/python
"""

from __future__ import annotations

import json
import sys
import types
import unittest
from pathlib import Path

TRACE = Path(__file__).resolve().parents[2] / "testdata" / "ai-trace.json"


def _stub_browser_modules() -> None:
    """Stand in for the Pyodide runtime so main.py can be imported."""
    if "js" not in sys.modules:
        js = types.ModuleType("js")
        js.document = None
        js.window = None
        js.localStorage = None
        sys.modules["js"] = js

    if "pyodide" not in sys.modules:
        pyodide = types.ModuleType("pyodide")
        pyodide.__path__ = []  # mark it as a package
        sys.modules["pyodide"] = pyodide

        ffi = types.ModuleType("pyodide.ffi")
        ffi.create_proxy = lambda fn: fn
        sys.modules["pyodide.ffi"] = ffi

        http = types.ModuleType("pyodide.http")

        async def pyfetch(*_args, **_kwargs):  # pragma: no cover - never called
            raise RuntimeError("no network in tests")

        http.pyfetch = pyfetch
        sys.modules["pyodide.http"] = http


_stub_browser_modules()
sys.path.insert(0, str(Path(__file__).resolve().parent))

import main  # noqa: E402  - needs the stubs above


def build_game(sample: dict) -> main.Game:
    """Rebuild the board state a recorded decision was made from."""
    cols, rows = sample["cols"], sample["rows"]
    game = main.Game(cols=cols, rows=rows)
    game.snake = [(cell % cols, cell // cols) for cell in sample["snake"]]
    game.occupied = set(game.snake)
    game.food = (
        None if sample["food"] is None
        else (sample["food"] % cols, sample["food"] // cols)
    )
    game.direction = main.DIRS[sample["dir"]]
    game.alive = True
    return game


class DemoAITest(unittest.TestCase):
    """Replays recorded board states and demands identical answers."""

    @classmethod
    def setUpClass(cls) -> None:
        cls.samples = json.loads(TRACE.read_text(encoding="utf-8"))
        assert cls.samples, "trace is empty"

    def test_matches_reference(self) -> None:
        seen = {0: 0, 1: 0, 2: 0}
        for i, sample in enumerate(self.samples):
            decision = main.decide(build_game(sample))
            seen[decision.tier] += 1
            where = f"sample {i} ({sample['cols']}x{sample['rows']}, len {len(sample['snake'])})"

            self.assertEqual(decision.tier, sample["tier"], f"{where}: tier")
            self.assertEqual(
                main.DIRS.index(decision.direction), sample["chosen"], f"{where}: direction"
            )
            self.assertEqual(len(decision.reachable), sample["reachable"], f"{where}: reachable")
            self.assertEqual(len(decision.path), sample["path"], f"{where}: path")

        # The trace is only worth something if it exercises all three tiers.
        for tier, name in ((0, "tier1"), (1, "tier2"), (2, "fallback")):
            self.assertGreater(seen[tier], 0, f"trace never reached {name}")


class GameRulesTest(unittest.TestCase):
    """Covers the parts of the engine the trace cannot."""

    def test_opening_position(self) -> None:
        game = main.Game()
        game.start()
        self.assertEqual(len(game.snake), 3)
        self.assertTrue(game.alive)
        self.assertEqual(game.direction, main.RIGHT)
        self.assertIsNotNone(game.food)

    def test_eating_grows_and_scores(self) -> None:
        game = main.Game(cols=10, rows=10)
        game.start()
        hx, hy = game.snake[0]
        game.food = (hx + 1, hy)
        ate, died, _ = game.step()
        self.assertTrue(ate)
        self.assertFalse(died)
        self.assertEqual(len(game.snake), 4)
        self.assertEqual(game.score, 1)

    def test_reversal_is_ignored(self) -> None:
        game = main.Game(cols=10, rows=10)
        game.start()
        game.queue_dir(main.LEFT)
        _, died, _ = game.step()
        self.assertFalse(died, "a reversal should be ignored, not fatal")

    def test_wall_is_fatal(self) -> None:
        game = main.Game(cols=10, rows=10)
        game.start()
        for _ in range(20):
            if not game.alive:
                break
            game.step()
        self.assertFalse(game.alive)

    def test_tail_is_fatal(self) -> None:
        # The TypeScript engine tests the whole snake before popping the tail,
        # so stepping onto your own tail is a collision.
        game = main.Game(cols=8, rows=8)
        game.start()
        game.snake = [(2, 2), (2, 3), (3, 3), (3, 2)]
        game.occupied = set(game.snake)
        game.direction = main.UP
        game.food = (7, 7)
        game.queue_dir(main.RIGHT)
        _, died, _ = game.step()
        self.assertTrue(died)


if __name__ == "__main__":
    unittest.main()
