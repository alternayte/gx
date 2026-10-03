; Injections for Gx (REQ-TLS-06): Go for expressions and the props and
; signals blocks, TypeScript for `lang="ts"` scripts, JavaScript and CSS for
; the rest.

((script_element
  (start_tag
    (attribute
      (attribute_name) @_name
      (quoted_attribute_value (attribute_value) @_lang)))
  (raw_text) @injection.content)
 (#eq? @_name "lang")
 (#match? @_lang "^ts$")
 (#set! injection.language "typescript"))

((script_element
  (raw_text) @injection.content)
 (#set! injection.language "javascript"))

((style_element
  (raw_text) @injection.content)
 (#set! injection.language "css"))

((expression) @injection.content
 (#set! injection.language "go"))

((go_block) @injection.content
 (#set! injection.language "go"))
