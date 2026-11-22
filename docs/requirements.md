# Codebreaker (Mastermind) - Terminal Game Requirements

**Version:** 1.1.0 · **Updated:** 2026-11-21
**Rules source of truth:** `references/Instructions_-_Code_Breaker_Bilingual.pdf` (the box rules). Where the English and French text differ, the English text governs unless a decision below says otherwise.

## Overview
A terminal-based implementation of Code Breaker (Mastermind-style). Two players take turns as Code Maker and Code Breaker. The Breaker deduces a hidden 5-color code from feedback; points are scored on a 30-hole track, and the first player to reach 30 wins.

## Decisions Log

| # | Topic | Decision | Source |
|---|---|---|---|
| D1 | Language | Go. Overrides the AGENTS.md default (Python) for this project: single-binary CLI distribution. | Project owner |
| D2 | Win condition (round) | Round is solved when a guess scores **5** exact matches (previous text said 4 — bug). | PDF §5–7 (5-peg code) |
| D3 | Rows per round | 12 guesses maximum. Round ends unsolved after row 12. | PDF §6–7 |
| D4 | Scoring | The **Code Breaker** scores by rows used (table below). The Code Maker scores nothing. Replaces the old "Code Maker +1 per guess" rule. | PDF "Round Scoring" |
| D5 | Match end | First player to reach **30 or more** points wins, immediately after the round that gets them there. Only one player scores per round, so ties cannot occur. | PDF "How to Win" |
| D6 | Role order | A coin flip (random) picks the first Code Maker. Roles swap after every round. | PDF §1, §8 |
| D7 | Code rules | 5 positions, 8 colors, each color at most once, no blanks. | PDF §2 (normal rules) |
| D8 | Feedback | Computed automatically by the game, not entered by the Code Maker. Prevents errors and cheating, and is required when the computer is the Code Maker. | Deviation from PDF §6 (software necessity) |
| D9 | Feedback meaning | Black key peg (shown as ●) = right color, right position. White key peg (shown as △) = right color, wrong position. Shown as counts in a fixed order, never tied to specific positions. | PDF §6 (English; the French text swaps black/white) |
| D10 | Hidden code | The Code Maker's code is hidden from the Breaker (the box's lid). In Human vs Human, the code is typed hidden and the screen is cleared before handoff. | PDF §2 |
| D11 | Scoring pegs | Players are identified as **Blue** and **Red** (their scoring-peg colors), alongside their names. | PDF §1 |
| D12 | Advanced options | Duplicate colors and blank positions are **out of scope**. | PDF "Advanced Options" (optional variant) |
| D13 | Single Game | One round. Roles are chosen in the menu or coin-flipped. The result shows solved/unsolved, rows used, and the points the bracket would award. | Project owner (PDF silent) |
| D14 | Repeated guesses | A guess identical to an earlier one in the same round is rejected as invalid input. It does not use a row. | PDF §7 (French: "propositions différentes à chaque fois") |
| D15 | First-Breaker advantage | Accepted. The match ends immediately when a player reaches 30, as D5 says; there is no "equal turns" rule. | PDF "How to Win" |
| D16 | Endless matches | No round cap. The quit command ends the match at any time and shows the standings. | Project owner (PDF silent) |
| D17 | Computer vs Computer | Not offered in the menu. Used only for automated tests and demos. | Project owner (PDF silent) |

## Core Game Parameters

| Parameter | Value |
|-----------|-------|
| Code Length (Holes) | 5 positions |
| Number of Colors | 8 distinct colors |
| Duplicates Allowed | No |
| Blanks | No |
| Rows per Round | 12 |
| Round Won | 5 exact matches (●●●●●) |
| Match Won | First player to 30+ points |
| Player Types | Human vs Computer, Human vs Human |

## Color Palette (8 Colors)

| Abbrev | Color | Notes |
|---|---|---|
| K | Black | `K` avoids clashing with Blue |
| W | White | |
| R | Red | |
| B | Blue | |
| G | Green | |
| Y | Yellow | |
| P | Pink | "Fuchsia" in the French rules |
| O | Orange | |

## Gameplay

### Match Setup
1. Choose the mode and the opponent (Human or Computer).
2. Each player gets a name and a scoring color (Blue or Red).
3. A coin flip decides who is Code Maker first.

### Round Flow
1. The Code Maker sets a secret code of 5 different colors, hidden from the Breaker.
2. The Code Breaker enters a guess. The game validates it and shows feedback.
3. All earlier guesses and their feedback stay visible for the whole round.
4. The round ends when the Breaker gets 5 exact matches, or after row 12.
5. The secret is revealed, the Breaker's points are added, and the scores are shown.
6. Unless someone has reached 30, roles swap and a new round begins.

### Input Format
- Guesses use color letters: `K W R B G Y P O`, case-insensitive.
- Accepted forms: `RGYBK`, `R G Y B K`, `R,G,Y,B,K`.

### Input Validation
Invalid input shows a clear error and asks again. **It does not use up a row.** Errors are checked and reported in this order, one per attempt:
1. Unknown letter (report the character and position)
2. Wrong length (report the count entered)
3. Repeated color (report the color and both positions)
4. Repeat of an earlier guess in this round (report which row; D14)

### Feedback Display
- `●` = right color, right position (black key peg)
- `△` = right color, wrong position (white key peg)
- Exact matches are listed first, then partial matches, then empty slots, so the order never reveals which peg matched.

## Scoring

The Code Breaker advances by the number of rows used to solve the code:

| Solved in row | Points |
|---|---|
| 1–4 | 12 |
| 5–6 | 10 |
| 7–8 | 8 |
| 9–10 | 6 |
| 11–12 | 4 |
| Not solved | 0 |

The Code Maker earns no points for the round. Points carry over across rounds. The match ends as soon as a player's total reaches 30 or more.

## Game Modes

### Match (main mode)
Rounds are played with alternating roles until one player reaches 30 points (D5).

### Single Game
One round (D13). Roles are picked in the menu, or by coin flip. The round follows the normal rules (12 rows). The result screen shows whether the code was solved, the rows used, and the points the bracket would award. There is no match total.

## User Interface Requirements

1. **Menu System**
   - Select the mode and the opponent (Human/Computer)
   - Enter player names
   - Confirm the start and show the coin-flip result

2. **Game Screen**
   - Show the round number, who is Maker and who is Breaker, and the current row (e.g. `Row 3/12`)
   - Show both players' scores out of 30 (e.g. `Blue 18/30 · Red 10/30`)
   - Show all earlier guesses with their feedback
   - Show a color legend and the guess prompt
   - At the end of a round: reveal the secret, show rows used and points earned
   - At the end of the match: show the winner and the final scores

3. **Visual Elements**
   - Unicode ● and △ for feedback
   - Each colored peg also shows its letter, so the game is playable without color
   - ANSI color output, turned off for `NO_COLOR`, `TERM=dumb`, or output that isn't a terminal

4. **Human vs Human**
   - The secret is typed hidden and confirmed by typing it again
   - The screen is cleared and a "pass the keyboard" prompt is shown before the Breaker's turn

5. **Error Handling**
   - Friendly messages for invalid letters, wrong length, and repeated colors
   - A quit command (`q`) works at any prompt and shows the current standings

## Computer Player
- **As Code Maker:** picks a random valid code.
- **As Code Breaker:** required, because roles swap. Must use only the feedback it has received, and must solve every possible code within 12 rows. A simple consistent-guess solver is enough for v1.

## Technical Requirements
- **Language:** Go 1.22+ (D1)
- **Platform:** Cross-platform (Linux, macOS, Windows)
- **Dependencies:** Standard library plus `golang.org/x/term` (hidden input, terminal detection)
- **Architecture:** Modular packages for:
  - Game rules (codes, validation, feedback)
  - Game state (rounds, match, scoring)
  - Computer solver
  - Input handling
  - UI rendering
- **Development:** Test-first. Randomness is seedable so games are reproducible.
- **Learning tool:** The code is heavily commented, because this project is also used to learn Go. Every package and every identifier has a doc comment. Go idioms are explained with `// Go note:` comments. Tests say what they prove. Readable code is preferred over clever code. Details are in `implementation-plan.md` §1.

## Out of Scope
- Advanced Options: duplicate colors and blanks (D12)
- Persistent high scores
- Sound effects
- Custom color counts or code lengths
- Stronger AI (Knuth/minimax): a possible later enhancement

## Open Questions
None. All earlier questions are resolved as D13–D17.

## Changelog
- **1.1.0** (2026-11-21): Added the learning-tool commenting requirement.
- **1.0.0** (2026-11-21): Open questions resolved as D13–D17: Single Game is one round, repeated guesses are rejected, the immediate win at 30 is kept, there is no round cap, and Computer vs Computer is for tests only. Requirements baselined.
- **0.2.0** (2026-11-21): Rules aligned to the box PDF (D2–D12). Language fixed as Go (D1). Scoring changed to the Breaker row brackets with a first-to-30 win. Added 12-row limit, coin flip, automatic feedback, and open questions.
- **0.1.0**: Initial draft.
