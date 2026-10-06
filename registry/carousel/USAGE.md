# Carousel

A row of slides that the user moves through one at a time.

## Usage

```gx
<carousel.Carousel id="photos" label="Photos">
  <carousel.CarouselItem><img src="/one.jpg" alt="The harbour" /></carousel.CarouselItem>
  <carousel.CarouselItem><img src="/two.jpg" alt="The old town" /></carousel.CarouselItem>
</carousel.Carousel>
```

The slides are server HTML in a scroll container with CSS scroll snap.
The island adds the previous and next buttons and tells a screen reader the position.
With no script the user scrolls the slides with touch, a wheel or the arrow keys.

## Do

- Give the carousel a `Label` that names its content.
- Give each image an `alt` text.
- Keep the count of slides small. Each slide loads with the page.

## Don't

- Do not move the slides on a timer.
- Do not put the only copy of important content on a later slide.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the slides, then to the previous and next buttons. |
| Arrow Left, Arrow Right | Moves to the previous or next slide. A vertical carousel uses Arrow Up and Arrow Down. |
| Home, End | Moves to the first or last slide. |
| Enter, Space | Runs the button that has the focus. |
