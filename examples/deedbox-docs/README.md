# Deedbox docs

A Gx port of the Deedbox documentation. The Deedbox docs are MIT licensed:
https://github.com/alternayte/deedbox

The content under `content/docs` was converted from the Starlight project
with:

    gx import starlight --out content/docs ../deedbox/site

Run the site:

    gx dev -main .

Export the static site:

    gx export -main . --out dist

The parity suite is `just parity-docs`.
