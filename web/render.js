// render.js draws the browser version of Codebreaker.
//
// It holds no game logic. Go (codebreaker.wasm) decides everything and
// calls codebreakerRender with a View: a complete description of the
// screen, as JSON (see internal/web/view.go). This file turns that View
// into HTML, and sends each tap or key press back to Go as an Event by
// calling codebreakerSend (see internal/web/event.go).
//
// Because every View is complete, drawing it from scratch each time is
// always right, and there's no page state to get out of step with the
// game.

"use strict";

// The 8 colors in palette order, as in internal/code/color.go. The page
// needs them to draw the palette buttons; their CSS colors are in
// style.css (.peg-K and so on).
const COLORS = [
  ["K", "Black"], ["W", "White"], ["R", "Red"], ["B", "Blue"],
  ["G", "Green"], ["Y", "Yellow"], ["P", "Pink"], ["O", "Orange"],
];

// The symbols and spoken labels for key pegs.
const KEYS = {
  exact: ["●", "right color, right spot"],
  partial: ["△", "right color, wrong spot"],
  none: ["·", "no match"],
};

const app = document.getElementById("app");

// view is the View on screen, which the keyboard handler needs.
let view = null;

// send passes one event to Go.
function send(event) {
  globalThis.codebreakerSend(JSON.stringify(event));
}

// el builds an element: el("p", {class: "muted"}, "text", child, ...).
// Attributes starting with "on" become event listeners.
function el(tag, attrs, ...children) {
  const node = document.createElement(tag);
  for (const [name, value] of Object.entries(attrs || {})) {
    if (name.startsWith("on")) {
      node.addEventListener(name.slice(2), value);
    } else if (value !== false && value != null) {
      node.setAttribute(name, value === true ? "" : value);
    }
  }
  // Children may be nested lists (from map), to any depth.
  for (const child of children.flat(Infinity)) {
    if (child != null && child !== false) {
      node.append(child);
    }
  }
  return node;
}

// Go calls this with each View.
globalThis.codebreakerRender = function (json) {
  view = JSON.parse(json);
  const screen = SCREENS[view.screen] || drawText;
  // Drawing functions may leave a gap (null) for a part a screen doesn't
  // have, such as a menu; those are skipped.
  app.replaceChildren(...screen(view).filter((part) => part != null));
  // Put the keyboard focus somewhere useful: the name box, or the first
  // menu choice, so Enter works at once.
  const focus = app.querySelector("[data-focus]");
  if (focus) {
    focus.focus();
    // In the name box, select the remembered name, so typing replaces it
    // and Enter keeps it.
    if (focus.tagName === "INPUT") {
      focus.select();
    }
  }
};

// The drawing function for each screen. Each returns a list of elements.
const SCREENS = {
  name: drawName,
  menu: drawMenu,
  board: drawBoard,
  secret: drawBoard,
  summary: drawBoard,
  result: drawBoard,
  help: drawHelp,
  handoff: drawHandoff,
};

// lines draws a View's short text lines as paragraphs.
function lines(v) {
  return (v.lines || []).map((line) => el("p", {}, line));
}

// topbar is the title with the Help button and, while playing, Quit.
function topbar(title, withQuit) {
  return el("div", { class: "topbar" },
    el("h1", {}, title),
    el("button", { class: "small", "aria-label": "Help", onclick: () => send({ kind: "help" }) }, "?"),
    withQuit && el("button", { class: "small", onclick: () => send({ kind: "quit" }) }, "Quit"),
  );
}

// choices draws a menu's choices as buttons.
function choices(menu) {
  if (!menu) {
    return null;
  }
  return el("div", { class: "choices" },
    menu.choices.map((label, i) =>
      el("button", {
        class: i === 0 ? "primary" : null,
        "data-focus": i === 0,
        onclick: () => send({ kind: "choose", choice: i }),
      }, label)),
  );
}

// drawName asks for a player's name. The last name typed on each name
// screen (first player, second player) is remembered on this device, for
// convenience; nothing leaves the device.
function drawName(v) {
  const key = v.title;
  const input = el("input", {
    type: "text",
    maxlength: 40,
    autocomplete: "nickname",
    "aria-label": "Name",
    placeholder: "Name",
    "data-focus": true,
    value: loadName(key),
  });
  const form = el("form", {
    class: "name",
    onsubmit: (e) => {
      e.preventDefault();
      saveName(key, input.value);
      send({ kind: "text", text: input.value });
    },
  }, input, el("button", { class: "primary", type: "submit" }, "Continue"));
  return [topbar(v.title, false), ...lines(v), form,
    el("p", { class: "message", role: "alert" }, v.message || "")];
}

// drawMenu draws a title, some text and the choices.
function drawMenu(v) {
  return [topbar(v.title, false), ...lines(v), choices(v.menu)];
}

// drawText draws a View with only text, such as an error from Go.
function drawText(v) {
  return [el("h1", {}, v.title || ""), ...lines(v)];
}

// drawHandoff draws the "Pass the device" card between two people. It
// shows no board, so nothing of the secret is on screen while the device
// changes hands (D21).
function drawHandoff(v) {
  return [
    el("div", { class: "topbar" },
      el("span", { class: "spacer" }),
      el("button", { class: "small", "aria-label": "Help", onclick: () => send({ kind: "help" }) }, "?"),
      el("button", { class: "small", onclick: () => send({ kind: "quit" }) }, "Quit")),
    el("div", { class: "handoff" },
      el("h1", {}, v.title),
      ...lines(v),
      el("button", { class: "primary", "data-focus": true, onclick: () => send({ kind: "ready" }) }, "I'm ready")),
  ];
}

// drawHelp draws the help sheet, with Close.
function drawHelp(v) {
  return [
    el("h1", {}, v.title),
    ...lines(v),
    el("div", { class: "choices" },
      el("button", { class: "primary", "data-focus": true, onclick: () => send({ kind: "close" }) }, "Close")),
  ];
}

// drawBoard draws the board screens: playing or watching the computer
// play (board), making a secret (secret), the end of a round in a match
// (summary) and the end of a game (result).
function drawBoard(v) {
  const b = v.board;
  const playing = v.screen === "board" || v.screen === "secret";
  const title = v.title || b.title;
  const out = [topbar(title, playing)];

  if (v.screen !== "secret") {
    out.push(seats(b));
  }
  // The board goes in one column and the text and controls in another.
  // On a narrow screen they stack; on a wide one (a Chromebook) they sit
  // side by side, so all 12 rows and the palette fit without scrolling.
  const left = el("div", { class: "col-board" },
    el("div", { class: "board" }, boardRows(v)),
    v.screen !== "secret" && el("p", { class: "legend" },
      "● right color, right spot · △ right color, wrong spot",
      b.easy && el("br"),
      b.easy && "Easy: each symbol is in the same spot as the peg it scores."));
  const right = el("div", { class: "col-controls" },
    ...lines(v),
    el("p", { class: "message", role: "alert" }, v.message || ""),
    v.input.palette && palette(v),
    v.input.skip && el("div", { class: "choices" },
      el("button", { "data-focus": true, onclick: () => send({ kind: "skip" }) }, "Skip to the end of the round")),
    choices(v.menu));
  out.push(el("div", { class: "play" }, left, right));
  return out;
}

// seats shows who makes and who breaks the code, and the scores in a
// match.
function seats(b) {
  const who = (s) => el("span", { class: "seat-" + s.peg }, s.name);
  const scores = b.scores
    ? el("span", {}, b.scores.map((s, i) => [
      i > 0 ? " · " : null, who(s), " " + s.score + "/" + b.target,
    ]))
    : null;
  return el("div", { class: "seats" },
    el("span", {}, "Maker ", who(b.maker), " · Breaker ", who(b.breaker)),
    scores);
}

// boardRows draws every row: played rows, then the draft in the next row,
// then empty rows up to the limit, and the secret once it's revealed.
function boardRows(v) {
  const b = v.board;
  const rows = [];
  for (const r of b.rows || []) {
    rows.push(row(r.row, r.guess, r.feedback, ""));
  }
  if (b.draft) {
    // A secret being made isn't a numbered row.
    const label = v.screen === "secret" ? "Code" : "";
    rows.push(row(rows.length + 1, b.draft, null, "current", label));
  }
  if (v.screen !== "secret") {
    for (let n = rows.length + 1; n <= b.maxRows; n++) {
      rows.push(row(n, [], null, ""));
    }
  }
  if (b.secret) {
    rows.push(el("div", { class: "secret-label" }, "Secret code"));
    rows.push(row("", b.secret, null, "secret", ""));
  }
  return rows;
}

// row draws one line of the board.
function row(number, pegs, feedback, cls, label) {
  const slots = [];
  for (let i = 0; i < 5; i++) {
    slots.push(peg(pegs[i]));
  }
  const keys = feedback
    ? feedback.map((k) => el("span", { class: "key key-" + k, "aria-label": KEYS[k][1] }, KEYS[k][0]))
    : [];
  return el("div", { class: "row " + cls },
    el("span", { class: "rownum" }, label || String(number)),
    el("span", { class: "pegs" }, slots),
    el("span", { class: "keys", role: feedback ? "group" : null, "aria-label": feedback ? "feedback" : null }, keys));
}

// peg draws one code peg, or an empty hole.
function peg(p) {
  if (!p || !p.letter) {
    return el("span", { class: "peg", "aria-label": "empty" });
  }
  return el("span", { class: "peg peg-" + p.letter, "aria-label": p.name }, p.letter);
}

// palette draws the color buttons, then Undo and Submit.
function palette(v) {
  return el("div", {},
    el("div", { class: "palette" },
      COLORS.map(([letter, name]) =>
        el("button", {
          class: "peg-" + letter,
          "aria-label": name,
          onclick: () => send({ kind: "color", letter }),
        }, letter))),
    el("div", { class: "actions" },
      el("button", { disabled: !v.input.undo, onclick: () => send({ kind: "undo" }) }, "Undo"),
      el("button", { class: "primary", disabled: !v.input.submit, onclick: () => send({ kind: "submit" }) }, "Submit")));
}

// The keyboard: color letters, Backspace to undo, Enter to submit, ? for
// help and Escape to close it. Typing in the name box is left alone, and
// on other screens Enter presses the focused button (such as Skip, or the
// first menu choice), as browsers do.
document.addEventListener("keydown", (e) => {
  if (!view || e.ctrlKey || e.metaKey || e.altKey || e.target.tagName === "INPUT") {
    return;
  }
  if (view.screen === "help") {
    if (e.key === "Escape") {
      send({ kind: "close" });
    }
    return;
  }
  if (e.key === "?") {
    send({ kind: "help" });
  } else if (view.input.palette && e.key.length === 1 && COLORS.some(([l]) => l === e.key.toUpperCase())) {
    send({ kind: "color", letter: e.key });
  } else if (view.input.palette && e.key === "Backspace") {
    e.preventDefault();
    send({ kind: "undo" });
  } else if (view.input.palette && e.key === "Enter") {
    e.preventDefault();
    send({ kind: "submit" });
  }
});

// loadName and saveName remember the last name typed on each name
// screen, keyed by its title, on this device. Storage can be missing or
// blocked (private windows, some settings), so both fail quietly.
function loadName(key) {
  try {
    return localStorage.getItem("codebreaker.name." + key) || "";
  } catch {
    return "";
  }
}

function saveName(key, name) {
  try {
    localStorage.setItem("codebreaker.name." + key, name.trim());
  } catch {
    // Not remembered; nothing else depends on it.
  }
}

// start loads the Go program and runs it. Go then calls
// codebreakerRender with the first screen.
async function start() {
  if (!("WebAssembly" in globalThis)) {
    app.replaceChildren(el("p", { class: "status" }, "This browser can't run Codebreaker (it needs WebAssembly)."));
    return;
  }
  try {
    // Go comes from wasm_exec.js.
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(fetch("codebreaker.wasm"), go.importObject);
    go.run(result.instance);
  } catch (err) {
    app.replaceChildren(el("p", { class: "status" }, "Codebreaker couldn't start: " + err));
  }
}

start();
