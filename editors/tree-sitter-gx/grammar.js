/**
 * @file Gx grammar for tree-sitter, forked from tree-sitter-html
 * @author Max Brunsfeld <maxbrunsfeld@gmail.com>
 * @author Amaan Qureshi <amaanq12@gmail.com>
 * @license MIT
 */

/// <reference types="tree-sitter-cli/dsl" />
// @ts-check

// The grammar follows docs/grammar.md and internal/compiler/parser.go. The
// rules that depend on the line (control lines, statement lines, the closing
// brace of a block, text) are tokens of the external scanner.

// The pieces of Go source that a props or signals block holds outside
// nested braces, comments and strings.
const goBlockText = token(choice(/[^{}\/"`\s][^{}\/"`\n]*/, '/', '"', '`'));

module.exports = grammar({
  name: 'gx',

  extras: $ => [
    $.comment,
    /\s+/,
  ],

  externals: $ => [
    $._start_tag_name,
    $._script_start_tag_name,
    $._style_start_tag_name,
    $._end_tag_name,
    $.erroneous_end_tag_name,
    '/>',
    $._implicit_end_tag,
    $.raw_text,
    $.comment,
    $.text,
    $._go_code,
    $._package,
    $._import,
    $._props,
    $._signals,
    $._if,
    $._for,
    $._switch,
    $._else,
    $._case,
    $._default,
    $._control_header,
    $._block_close,
    $.statement,
    $.fragment_parameters,
  ],

  rules: {
    document: $ => seq(
      optional($.package_clause),
      repeat($.import_declaration),
      optional(alias($._props_block, $.go_block)),
      optional(alias($._signals_block, $.go_block)),
      repeat($._node),
    ),

    package_clause: $ => seq(alias($._package, 'package'), $.package_name),

    package_name: _ => /[A-Za-z_][A-Za-z0-9_]*/,

    import_declaration: $ => seq(
      alias($._import, 'import'),
      choice(
        $.import_spec,
        seq('(', repeat(choice($.import_spec, $.line_comment, $.block_comment)), ')'),
      ),
    ),

    import_spec: $ => seq(optional($.import_alias), $.go_string),

    import_alias: _ => /[A-Za-z_.][A-Za-z0-9_]*/,

    // The props and signals blocks hold Go struct fields. A // comment
    // above a field is its description. A brace in a comment or in a
    // string does not open or close the block.
    _props_block: $ => seq(alias($._props, 'props'), $._go_braces),

    _signals_block: $ => seq(alias($._signals, 'signals'), $._go_braces),

    _go_braces: $ => seq(
      '{',
      repeat(choice(
        $._go_braces,
        $.line_comment,
        $.go_string,
        goBlockText,
      )),
      '}',
    ),

    line_comment: _ => token(seq('//', /[^\n]*/)),

    block_comment: _ => token(seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/')),

    go_string: _ => token(choice(
      seq('"', repeat(choice(/[^"\\\n]/, /\\./)), '"'),
      seq('`', /[^`]*/, '`'),
    )),

    _node: $ => choice(
      $.expression,
      $.entity,
      $.text,
      $.element,
      $.script_element,
      $.style_element,
      $.erroneous_end_tag,
      $.if_statement,
      $.for_statement,
      $.switch_statement,
      $.statement,
    ),

    // A Go expression or a client expression. The scanner reads the body:
    // a brace in a string or in a comment does not close the expression.
    expression: $ => seq('{', optional(alias($._go_code, $.go_code)), '}'),

    // A control line starts at a line start and ends with {. The body ends
    // at a } that is the first character on its line.
    if_statement: $ => seq(
      alias($._if, 'if'),
      optional(alias($._control_header, $.go_code)),
      $.block,
      optional($.else_clause),
    ),

    else_clause: $ => seq(
      alias($._else, 'else'),
      choice($.block, $.if_statement),
    ),

    for_statement: $ => seq(
      alias($._for, 'for'),
      optional(alias($._control_header, $.go_code)),
      $.block,
    ),

    switch_statement: $ => seq(
      alias($._switch, 'switch'),
      optional(alias($._control_header, $.go_code)),
      '{',
      repeat(choice($.line_comment, $.block_comment)),
      repeat(choice($.case_clause, $.default_clause)),
      alias($._block_close, '}'),
    ),

    case_clause: $ => seq(
      alias($._case, 'case'),
      optional(alias(/[^:\s][^:]*/, $.go_code)),
      ':',
      repeat($._node),
    ),

    default_clause: $ => seq(
      alias($._default, 'default'),
      ':',
      repeat($._node),
    ),

    block: $ => seq('{', repeat($._node), alias($._block_close, '}')),

    element: $ => choice(
      seq(
        $.start_tag,
        repeat($._node),
        choice($.end_tag, $._implicit_end_tag),
      ),
      $.self_closing_tag,
    ),

    script_element: $ => seq(
      alias($.script_start_tag, $.start_tag),
      optional($.raw_text),
      $.end_tag,
    ),

    style_element: $ => seq(
      alias($.style_start_tag, $.start_tag),
      optional($.raw_text),
      $.end_tag,
    ),

    start_tag: $ => seq(
      '<',
      alias($._start_tag_name, $.tag_name),
      repeat($._attribute),
      '>',
    ),

    script_start_tag: $ => seq(
      '<',
      alias($._script_start_tag_name, $.tag_name),
      repeat($._attribute),
      '>',
    ),

    style_start_tag: $ => seq(
      '<',
      alias($._style_start_tag_name, $.tag_name),
      repeat($._attribute),
      '>',
    ),

    self_closing_tag: $ => seq(
      '<',
      alias($._start_tag_name, $.tag_name),
      repeat($._attribute),
      '/>',
    ),

    end_tag: $ => seq(
      '</',
      alias($._end_tag_name, $.tag_name),
      '>',
    ),

    erroneous_end_tag: $ => seq(
      '</',
      $.erroneous_end_tag_name,
      '>',
    ),

    _attribute: $ => choice(
      $.attribute,
      $.fragment_attribute,
      $.spread_attribute,
    ),

    attribute: $ => seq(
      $.attribute_name,
      optional(seq(
        '=',
        choice(
          $.expression,
          $.attribute_value,
          $.quoted_attribute_value,
        ),
      )),
    ),

    // A fragment attribute: #name or #name(params).
    fragment_attribute: $ => seq(
      '#',
      alias(/[A-Za-z0-9_-]+/, $.attribute_name),
      optional($.fragment_parameters),
    ),

    // A spread attribute: {...expr}.
    spread_attribute: $ => seq(
      '{',
      '...',
      optional(alias($._go_code, $.go_code)),
      '}',
    ),

    attribute_name: _ => /[^<>"'\/={}#\s][^<>"'\/={}\s]*/,

    attribute_value: _ => /[^>"'{\s][^>\s]*/,

    // An entity can be named, numeric (decimal), or numeric (hexacecimal). The
    // longest entity name is 29 characters long, and the HTML spec says that
    // no more will ever be added.
    entity: _ => /&(#([xX][0-9a-fA-F]{1,6}|[0-9]{1,5})|[A-Za-z]{1,30});?/,

    quoted_attribute_value: $ => choice(
      seq('\'', optional(alias(/[^']+/, $.attribute_value)), '\''),
      seq('"', optional(alias(/[^"]+/, $.attribute_value)), '"'),
    ),
  },
});
