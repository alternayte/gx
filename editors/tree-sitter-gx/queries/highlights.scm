; Highlights for Gx (REQ-TLS-06). The tree is HTML-shaped; component tags,
; fragments, signals and directives carry the Gx meaning.

(tag_name) @tag
((tag_name) @type
 (#match? @type "^[A-Z]"))
((tag_name) @namespace
 (#match? @namespace "\\."))
(erroneous_end_tag_name) @tag.error
(doctype) @constant
(attribute_name) @attribute
((attribute_name) @keyword
 (#match? @keyword "^(show|text|key|transition)$"))
((attribute_name) @keyword
 (#match? @keyword "^(bind|class|attr|on):"))
(attribute_value) @string
(quoted_attribute_value) @string
(fragment_attribute (attribute_name) @function)
(expression) @embedded
(go_block) @embedded
(line_comment) @comment
(go_string) @string
(comment) @comment

[
  "<"
  ">"
  "</"
  "/>"
] @punctuation.bracket
