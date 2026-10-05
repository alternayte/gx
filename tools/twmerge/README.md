# tools/twmerge

`gx.Cx` merges classes with the semantics of
[tailwind-merge](https://github.com/dcastil/tailwind-merge) (MIT, Copyright
(c) 2021 Dany Castillo). These scripts keep the Go code in step with the
pinned release. Users never need them, and they never need node or Bun.

| File | Job |
| --- | --- |
| `gen.ts` | Writes `cxtable.go`. It serializes the class map that tailwind-merge builds from its default configuration. Run `just cx-table`. |
| `extract.ts` | Reads the test files of tailwind-merge and its documentation examples, and writes each default-configuration case as JSON. |
| `cases.sh` | Downloads the release from GitHub, checks that its `src` equals the pinned npm package, and runs `extract.ts`. Run `just cx-cases`. |

The pinned version is the `tailwind-merge` entry in
`tools/shadcn-ref/bun.lock`. `checks/cx-table.sh` fails when `cxtable.go` is
stale.

## Move to a new tailwind-merge release

1. Change the version in `tools/shadcn-ref`.
2. Run `just cx-table` and `just cx-cases`.
3. Compare `src/lib/merge-classlist.ts`, `parse-class-name.ts`,
   `sort-modifiers.ts`, `class-group-utils.ts` and `validators.ts` with the
   last release. `cx.go` is a port of these five files.
4. Change the file name in `TestREQ_STY_04_TailwindMergeSuite` and run it.

## Cases that are not ported

A case stays out when one argument is not a string: `gx.Cx` takes strings
only, with no arrays and no falsy values. The suites for a custom
configuration (`extendTailwindMerge`, `createTailwindMerge`, prefixes,
themes) stay out too: `gx.Cx` has one configuration, the default.
