# Motion stories

Each story drives the demo app through one piece of motion and asks for
frames at the moments worth looking at. They are how a change to motion is
checked without a screen: the frames come back side by side in one image.

```
rashin-app --demo --script search.story --shots 400,460,540 --png search.png
```

A line is a time in milliseconds and an event: `route`, `move`, `click`,
`secondary`, `press`, `release`, `scroll`, `key`, `type` or `wait`. Positions
are in dp. A story runs in real time, so what the demo plays by the clock, such
as an answer arriving, lands in the frames it would on screen. A `wait` early
on lets the page's data arrive before anything is pressed.

The `# Shots:` line of a story names the frames it was written for, and
`# Size:` the window when it is not 1280 by 820.
