# Codebreaker

A terminal version of the Code Breaker board game, written in Go. Two players take turns: one makes a secret code of 5 different colors, the other tries to break it in 12 guesses or fewer. The breaker scores more for solving in fewer rows, and the first player to reach 30 points wins.

> Status: early development. v0.2.0 is done: game rules and logic, fully tested. It isn't playable yet; v0.3.0 adds the terminal UI.

## Documents
- [Requirements](docs/requirements.md): the game rules and the decisions behind them
- [Implementation plan](docs/implementation-plan.md): architecture, milestones, and testing

## Development

Requires Go 1.24 or later.

```sh
make test   # run the tests
make lint   # go vet + staticcheck
make help   # list all targets
```

## Learning Go with this repo
The code is heavily commented on purpose. Each package's `doc.go` says which Go concepts it shows, and idioms are explained in comments that start with `Go note:`. To list them all:

```sh
grep -rn "Go note:" internal/
```
