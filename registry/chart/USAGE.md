# Chart

A bar, line or area chart.

## Usage

```gx
<chart.Chart id="visitors" title="Visitors" categories={p.Months} series={p.Visitors} />
<chart.Chart id="revenue" title="Revenue" kind={chart.Line} categories={p.Months} series={p.Revenue} />
```

The server renders the data as a table. The island draws the same data as SVG and keeps the table for a screen reader.
The colours of the series are the theme tokens `--chart-1` to `--chart-5`.
A patch with new data draws the chart again and keeps the active point.
With no script the page shows the table.

## Do

- Give the chart a `Title` that names the data.
- Give each series one value for each category.
- Use at most five series. The theme has five chart colours.

## Don't

- Do not use a chart for two or three numbers. Write them as text.
- Do not use colour as the only difference between two series that a reader must compare. Name them in the text.

## Keyboard

| Key | Action |
| --- | --- |
| Tab | Moves focus to the chart. |
| Arrow Right, Arrow Left | Shows the values of the next or previous category. |
| Home, End | Shows the values of the first or last category. |
| Escape | Hides the values. |
