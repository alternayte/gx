// The external scanner of the Gx grammar. It follows the rules of
// internal/compiler/parser.go that depend on context: tag names and the
// open element stack, raw text, and the constructs that depend on the line
// (control lines, statement lines, the closing brace of a block, text).

#include "tree_sitter/array.h"
#include "tree_sitter/parser.h"

#include <string.h>

enum TokenType {
    START_TAG_NAME,
    SCRIPT_START_TAG_NAME,
    STYLE_START_TAG_NAME,
    END_TAG_NAME,
    ERRONEOUS_END_TAG_NAME,
    SELF_CLOSING_TAG_DELIMITER,
    IMPLICIT_END_TAG,
    RAW_TEXT,
    COMMENT,
    TEXT,
    GO_CODE,
    PACKAGE,
    IMPORT,
    PROPS,
    SIGNALS,
    IF,
    FOR,
    SWITCH,
    ELSE,
    CASE,
    DEFAULT,
    CONTROL_HEADER,
    BLOCK_CLOSE,
    STATEMENT,
    FRAGMENT_PARAMETERS,
};

typedef enum {
    NORMAL,
    VOID,
    SCRIPT,
    STYLE,
} TagKind;

typedef Array(char) String;

// A tag name keeps its case: <Input> and <input> are different elements,
// and a closing tag must match its opening tag exactly.
typedef struct {
    TagKind kind;
    String name;
} Tag;

typedef struct {
    Array(Tag) tags;
} Scanner;

static const char *const VOID_TAGS[] = {
    "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr",
};

static inline void advance(TSLexer *lexer) { lexer->advance(lexer, false); }

static inline void skip(TSLexer *lexer) { lexer->advance(lexer, true); }

static inline bool is_space(int32_t c) { return c == ' ' || c == '\t' || c == '\n' || c == '\r'; }

static inline bool is_letter(int32_t c) { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'); }

static inline bool is_digit(int32_t c) { return c >= '0' && c <= '9'; }

static inline bool is_hex_digit(int32_t c) {
    return is_digit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F');
}

static inline bool is_ident_start(int32_t c) { return c == '_' || is_letter(c); }

static inline bool is_ident_char(int32_t c) { return c == '_' || is_letter(c) || is_digit(c); }

static inline bool is_tag_start(int32_t c) { return c == ':' || is_letter(c); }

static inline bool is_tag_char(int32_t c) { return c == '_' || c == '-' || c == '.' || is_letter(c) || is_digit(c); }

static inline char lower(char c) { return (c >= 'A' && c <= 'Z') ? (char)(c - 'A' + 'a') : c; }

// name_is compares a tag name with a lower-case word, ignoring case.
static bool name_is(const String *name, const char *word) {
    size_t length = strlen(word);
    if (name->size != length) {
        return false;
    }
    for (uint32_t i = 0; i < name->size; i++) {
        if (lower(name->contents[i]) != word[i]) {
            return false;
        }
    }
    return true;
}

static TagKind kind_for_name(const String *name) {
    if (name_is(name, "script")) {
        return SCRIPT;
    }
    if (name_is(name, "style")) {
        return STYLE;
    }
    for (unsigned i = 0; i < sizeof(VOID_TAGS) / sizeof(VOID_TAGS[0]); i++) {
        if (name_is(name, VOID_TAGS[i])) {
            return VOID;
        }
    }
    return NORMAL;
}

static bool name_eq(const String *a, const String *b) {
    return a->size == b->size && (a->size == 0 || memcmp(a->contents, b->contents, a->size) == 0);
}

static void pop_tag(Scanner *scanner) {
    Tag popped_tag = array_pop(&scanner->tags);
    array_delete(&popped_tag.name);
}

static void clear_tags(Scanner *scanner) {
    for (unsigned i = 0; i < scanner->tags.size; i++) {
        array_delete(&scanner->tags.contents[i].name);
    }
    array_clear(&scanner->tags);
}

static unsigned serialize(Scanner *scanner, char *buffer) {
    uint16_t tag_count = scanner->tags.size > UINT16_MAX ? UINT16_MAX : scanner->tags.size;
    uint16_t serialized_tag_count = 0;

    unsigned size = sizeof(tag_count);
    memcpy(&buffer[size], &tag_count, sizeof(tag_count));
    size += sizeof(tag_count);

    for (; serialized_tag_count < tag_count; serialized_tag_count++) {
        Tag tag = scanner->tags.contents[serialized_tag_count];
        unsigned name_length = tag.name.size;
        if (name_length > UINT8_MAX) {
            name_length = UINT8_MAX;
        }
        if (size + 2 + name_length >= TREE_SITTER_SERIALIZATION_BUFFER_SIZE) {
            break;
        }
        buffer[size++] = (char)tag.kind;
        buffer[size++] = (char)name_length;
        if (name_length > 0) {
            memcpy(&buffer[size], tag.name.contents, name_length);
        }
        size += name_length;
    }

    memcpy(&buffer[0], &serialized_tag_count, sizeof(serialized_tag_count));
    return size;
}

static void deserialize(Scanner *scanner, const char *buffer, unsigned length) {
    clear_tags(scanner);

    if (length > 0) {
        unsigned size = 0;
        uint16_t tag_count = 0;
        uint16_t serialized_tag_count = 0;

        memcpy(&serialized_tag_count, &buffer[size], sizeof(serialized_tag_count));
        size += sizeof(serialized_tag_count);

        memcpy(&tag_count, &buffer[size], sizeof(tag_count));
        size += sizeof(tag_count);

        array_reserve(&scanner->tags, tag_count);
        unsigned iter = 0;
        for (iter = 0; iter < serialized_tag_count; iter++) {
            Tag tag = {NORMAL, array_new()};
            tag.kind = (TagKind)buffer[size++];
            uint8_t name_length = (uint8_t)buffer[size++];
            if (name_length > 0) {
                array_extend(&tag.name, name_length, &buffer[size]);
            }
            size += name_length;
            array_push(&scanner->tags, tag);
        }
        // add unnamed tags if we didn't read enough, this is because the
        // buffer had no more room but we held more tags.
        for (; iter < tag_count; iter++) {
            Tag tag = {NORMAL, array_new()};
            array_push(&scanner->tags, tag);
        }
    }
}

// scan_tag_name reads a tag name: <div>, <Button>, <ui.Card>, <wa-button>
// and <:slot>.
static String scan_tag_name(TSLexer *lexer) {
    String tag_name = array_new();
    if (lexer->lookahead == ':') {
        array_push(&tag_name, ':');
        advance(lexer);
    }
    while (is_tag_char(lexer->lookahead)) {
        array_push(&tag_name, (char)lexer->lookahead);
        advance(lexer);
    }
    return tag_name;
}

// scan_html_comment reads a comment after its <!-- marker.
static bool scan_html_comment(TSLexer *lexer) {
    unsigned dashes = 0;
    while (!lexer->eof(lexer)) {
        switch (lexer->lookahead) {
            case '-':
                ++dashes;
                break;
            case '>':
                if (dashes >= 2) {
                    lexer->result_symbol = COMMENT;
                    advance(lexer);
                    lexer->mark_end(lexer);
                    return true;
                }
                dashes = 0;
                break;
            default:
                dashes = 0;
        }
        advance(lexer);
    }
    return false;
}

// scan_raw_text reads the body of a script or style element. The body ends
// at the closing tag of the element, in any case.
static bool scan_raw_text(Scanner *scanner, TSLexer *lexer) {
    if (scanner->tags.size == 0) {
        return false;
    }
    const String *name = &array_back(&scanner->tags)->name;

    lexer->mark_end(lexer);

    // matched counts the characters of "</name" that the input holds at
    // the current position.
    bool has_content = false;
    unsigned matched = 0;
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        bool matches;
        if (matched == 0) {
            matches = c == '<';
        } else if (matched == 1) {
            matches = c == '/';
        } else {
            matches = c < 0x80 && lower((char)c) == lower(name->contents[matched - 2]);
        }
        if (matches) {
            matched++;
            if (matched == name->size + 2) {
                break;
            }
            advance(lexer);
            continue;
        }
        if (matched > 0) {
            // The partial match is raw text. Test this character again:
            // it may start the closing tag.
            matched = 0;
            lexer->mark_end(lexer);
            has_content = true;
            continue;
        }
        advance(lexer);
        lexer->mark_end(lexer);
        has_content = true;
    }

    if (!has_content) {
        return false;
    }
    lexer->result_symbol = RAW_TEXT;
    return true;
}

// skip_quoted advances past a Go string or rune literal. It reports false
// when the literal has no end.
static bool skip_quoted(TSLexer *lexer) {
    int32_t quote = lexer->lookahead;
    advance(lexer);
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        advance(lexer);
        if (c == '\\' && quote != '`') {
            if (!lexer->eof(lexer)) {
                advance(lexer);
            }
            continue;
        }
        if (c == quote) {
            return true;
        }
    }
    return false;
}

// scan_go_code reads the body of a {...} expression up to its closing
// brace. A string or a comment holds any brace.
static bool scan_go_code(TSLexer *lexer) {
    while (is_space(lexer->lookahead)) {
        skip(lexer);
    }
    bool has_content = false;
    unsigned depth = 0;
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        if (c == '"' || c == '\'' || c == '`') {
            if (!skip_quoted(lexer)) {
                return false;
            }
            lexer->mark_end(lexer);
            has_content = true;
            continue;
        }
        if (c == '/') {
            advance(lexer);
            if (lexer->lookahead == '/') {
                while (!lexer->eof(lexer) && lexer->lookahead != '\n') {
                    advance(lexer);
                }
            } else if (lexer->lookahead == '*') {
                advance(lexer);
                bool star = false;
                bool closed = false;
                while (!lexer->eof(lexer)) {
                    int32_t d = lexer->lookahead;
                    advance(lexer);
                    if (star && d == '/') {
                        closed = true;
                        break;
                    }
                    star = d == '*';
                }
                if (!closed) {
                    return false;
                }
            }
            lexer->mark_end(lexer);
            has_content = true;
            continue;
        }
        if (c == '{') {
            depth++;
        } else if (c == '}') {
            if (depth == 0) {
                if (!has_content) {
                    return false;
                }
                lexer->result_symbol = GO_CODE;
                return true;
            }
            depth--;
        }
        advance(lexer);
        if (!is_space(c)) {
            lexer->mark_end(lexer);
            has_content = true;
        }
    }
    return false;
}

// scan_control_header reads the header of an if, for or switch line up to
// the { that opens the block.
static bool scan_control_header(TSLexer *lexer) {
    while (is_space(lexer->lookahead)) {
        skip(lexer);
    }
    bool has_content = false;
    int depth = 0;
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        if (c == '{' && depth == 0) {
            break;
        }
        if (c == '(' || c == '[') {
            depth++;
        } else if (c == ')' || c == ']') {
            depth--;
        }
        advance(lexer);
        if (!is_space(c)) {
            lexer->mark_end(lexer);
            has_content = true;
        }
    }
    if (!has_content) {
        return false;
    }
    lexer->result_symbol = CONTROL_HEADER;
    return true;
}

// scan_fragment_parameters reads the (params) of a fragment attribute.
static bool scan_fragment_parameters(TSLexer *lexer) {
    advance(lexer);
    unsigned depth = 1;
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        if (c == '"' || c == '\'' || c == '`') {
            if (!skip_quoted(lexer)) {
                return false;
            }
            continue;
        }
        advance(lexer);
        if (c == '(') {
            depth++;
        } else if (c == ')') {
            depth--;
            if (depth == 0) {
                lexer->mark_end(lexer);
                lexer->result_symbol = FRAGMENT_PARAMETERS;
                return true;
            }
        }
    }
    return false;
}

static bool scan_start_tag_name(Scanner *scanner, TSLexer *lexer) {
    if (!is_tag_start(lexer->lookahead)) {
        return false;
    }
    Tag tag = {NORMAL, scan_tag_name(lexer)};
    tag.kind = kind_for_name(&tag.name);
    array_push(&scanner->tags, tag);
    switch (tag.kind) {
        case SCRIPT:
            lexer->result_symbol = SCRIPT_START_TAG_NAME;
            break;
        case STYLE:
            lexer->result_symbol = STYLE_START_TAG_NAME;
            break;
        default:
            lexer->result_symbol = START_TAG_NAME;
            break;
    }
    return true;
}

static bool scan_end_tag_name(Scanner *scanner, TSLexer *lexer) {
    String tag_name = scan_tag_name(lexer);
    if (tag_name.size == 0) {
        array_delete(&tag_name);
        return false;
    }

    if (scanner->tags.size > 0 && name_eq(&array_back(&scanner->tags)->name, &tag_name)) {
        pop_tag(scanner);
        lexer->result_symbol = END_TAG_NAME;
    } else {
        lexer->result_symbol = ERRONEOUS_END_TAG_NAME;
    }

    array_delete(&tag_name);
    return true;
}

static bool scan_self_closing_tag_delimiter(Scanner *scanner, TSLexer *lexer) {
    advance(lexer);
    if (lexer->lookahead == '>') {
        advance(lexer);
        if (scanner->tags.size > 0) {
            pop_tag(scanner);
            lexer->result_symbol = SELF_CLOSING_TAG_DELIMITER;
        }
        return true;
    }
    return false;
}

// scan_unmatched_end_tag runs at a </ that the content of an element holds.
// When the tag closes an element below the top of the stack, the top
// element gets an implicit end. This keeps the tree sound while the user
// edits.
static bool scan_unmatched_end_tag(Scanner *scanner, TSLexer *lexer) {
    advance(lexer);
    String tag_name = scan_tag_name(lexer);
    bool implicit = false;
    if (scanner->tags.size > 0 && !name_eq(&array_back(&scanner->tags)->name, &tag_name)) {
        for (unsigned i = scanner->tags.size - 1; i > 0; i--) {
            if (name_eq(&scanner->tags.contents[i - 1].name, &tag_name)) {
                implicit = true;
                break;
            }
        }
    }
    array_delete(&tag_name);
    if (!implicit) {
        return false;
    }
    pop_tag(scanner);
    lexer->result_symbol = IMPLICIT_END_TAG;
    return true;
}

// at_entity runs after an &. It reports whether the & starts an entity. It
// consumes the characters it tests; they are text when it reports false.
static bool at_entity(TSLexer *lexer) {
    if (is_letter(lexer->lookahead)) {
        return true;
    }
    if (lexer->lookahead != '#') {
        return false;
    }
    advance(lexer);
    if (is_digit(lexer->lookahead)) {
        return true;
    }
    if (lexer->lookahead != 'x' && lexer->lookahead != 'X') {
        return false;
    }
    advance(lexer);
    return is_hex_digit(lexer->lookahead);
}

// scan_text reads the rest of a text token. The caller has consumed its
// first character. Text ends at a tag, an expression, an entity or the end
// of the line; the spaces at its end are not part of it.
static bool scan_text(TSLexer *lexer) {
    lexer->mark_end(lexer);
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        if (c == '\n' || c == '{') {
            break;
        }
        if (c == '<') {
            advance(lexer);
            if (is_tag_start(lexer->lookahead) || lexer->lookahead == '/') {
                break;
            }
            lexer->mark_end(lexer);
            continue;
        }
        if (c == '&') {
            advance(lexer);
            if (at_entity(lexer)) {
                break;
            }
            lexer->mark_end(lexer);
            continue;
        }
        advance(lexer);
        if (!is_space(c)) {
            lexer->mark_end(lexer);
        }
    }
    lexer->result_symbol = TEXT;
    return true;
}

// line_ends_with_brace reads to the end of the line and reports whether its
// last character that is not a space is {.
static bool line_ends_with_brace(TSLexer *lexer) {
    int32_t last = 0;
    while (!lexer->eof(lexer) && lexer->lookahead != '\n') {
        if (!is_space(lexer->lookahead)) {
            last = lexer->lookahead;
        }
        advance(lexer);
    }
    return last == '{';
}

#define WORD_SIZE 16

// scan_word reads an identifier into word and returns its length. A longer
// identifier is read to its end, and word holds its start.
static unsigned scan_word(TSLexer *lexer, char *word) {
    unsigned length = 0;
    while (is_ident_char(lexer->lookahead)) {
        if (length < WORD_SIZE - 1) {
            word[length] = (char)lexer->lookahead;
        }
        length++;
        advance(lexer);
    }
    word[length < WORD_SIZE ? length : WORD_SIZE - 1] = '\0';
    return length;
}

static inline bool emit(TSLexer *lexer, enum TokenType symbol) {
    lexer->mark_end(lexer);
    lexer->result_symbol = symbol;
    return true;
}

static inline void skip_blanks(TSLexer *lexer) {
    while (lexer->lookahead == ' ' || lexer->lookahead == '\t') {
        advance(lexer);
    }
}

// scan_statement runs at a line start after the first identifier of the
// line. It reads `name := expr` and `a, b := expr`. It reports false when
// the line is not a statement; the characters it consumed are then text.
static bool scan_statement(TSLexer *lexer) {
    for (;;) {
        skip_blanks(lexer);
        if (lexer->lookahead != ',') {
            break;
        }
        advance(lexer);
        lexer->mark_end(lexer);
        skip_blanks(lexer);
        if (!is_ident_start(lexer->lookahead)) {
            return false;
        }
        while (is_ident_char(lexer->lookahead)) {
            advance(lexer);
        }
        lexer->mark_end(lexer);
    }
    if (lexer->lookahead != ':') {
        return false;
    }
    advance(lexer);
    lexer->mark_end(lexer);
    if (lexer->lookahead != '=') {
        return false;
    }
    advance(lexer);
    lexer->mark_end(lexer);

    int depth = 0;
    while (!lexer->eof(lexer)) {
        int32_t c = lexer->lookahead;
        if (c == '\n' && depth == 0) {
            break;
        }
        if (c == '(' || c == '[' || c == '{') {
            depth++;
        } else if (c == ')' || c == ']' || c == '}') {
            depth--;
            if (depth < 0) {
                break;
            }
        }
        advance(lexer);
        if (!is_space(c)) {
            lexer->mark_end(lexer);
        }
    }
    lexer->result_symbol = STATEMENT;
    return true;
}

// scan_content reads one token where a body node can start.
static bool scan_content(Scanner *scanner, TSLexer *lexer, const bool *valid_symbols) {
    // A void element has no children and no closing tag.
    if (valid_symbols[IMPLICIT_END_TAG] && scanner->tags.size > 0 && array_back(&scanner->tags)->kind == VOID) {
        pop_tag(scanner);
        return emit(lexer, IMPLICIT_END_TAG);
    }

    // The token is at a line start when only spaces come before it on its
    // line. Every token ends at a character that is not a space, so the
    // line start is a line break in the skipped spaces or column zero.
    bool line_start = lexer->get_column(lexer) == 0;
    bool skipped = false;
    while (is_space(lexer->lookahead)) {
        if (lexer->lookahead == '\n') {
            line_start = true;
        }
        skipped = true;
        skip(lexer);
    }
    if (lexer->eof(lexer)) {
        return false;
    }

    int32_t c = lexer->lookahead;

    if (c == '<') {
        lexer->mark_end(lexer);
        advance(lexer);
        if (lexer->lookahead == '/') {
            return valid_symbols[IMPLICIT_END_TAG] && scan_unmatched_end_tag(scanner, lexer);
        }
        if (is_tag_start(lexer->lookahead)) {
            return false;
        }
        if (lexer->lookahead == '!') {
            advance(lexer);
            if (lexer->lookahead == '-') {
                advance(lexer);
                if (lexer->lookahead == '-') {
                    advance(lexer);
                    return scan_html_comment(lexer);
                }
            }
        }
        return scan_text(lexer);
    }

    if (c == '{') {
        // {/* ... */} is a comment. Any other { opens an expression.
        advance(lexer);
        if (lexer->lookahead != '/') {
            return false;
        }
        advance(lexer);
        if (lexer->lookahead != '*') {
            return false;
        }
        advance(lexer);
        unsigned matched = 0;
        while (!lexer->eof(lexer)) {
            int32_t d = lexer->lookahead;
            advance(lexer);
            if (d == '*') {
                matched = 1;
            } else if (d == '/' && matched == 1) {
                matched = 2;
            } else if (d == '}' && matched == 2) {
                return emit(lexer, COMMENT);
            } else {
                matched = 0;
            }
        }
        return false;
    }

    if (c == '}') {
        if (valid_symbols[BLOCK_CLOSE] && (line_start || !skipped)) {
            advance(lexer);
            return emit(lexer, BLOCK_CLOSE);
        }
        if (line_start) {
            return false;
        }
        advance(lexer);
        return scan_text(lexer);
    }

    if (c == '&') {
        advance(lexer);
        if (at_entity(lexer)) {
            return false;
        }
        return scan_text(lexer);
    }

    bool header = valid_symbols[PACKAGE] || valid_symbols[IMPORT] || valid_symbols[PROPS] || valid_symbols[SIGNALS];

    // A comment before the package clause, between imports or around the
    // props and signals blocks.
    if (c == '/' && header) {
        advance(lexer);
        if (lexer->lookahead == '/') {
            while (!lexer->eof(lexer) && lexer->lookahead != '\n') {
                advance(lexer);
            }
            return emit(lexer, COMMENT);
        }
        if (lexer->lookahead == '*') {
            advance(lexer);
            bool star = false;
            while (!lexer->eof(lexer)) {
                int32_t d = lexer->lookahead;
                advance(lexer);
                if (star && d == '/') {
                    break;
                }
                star = d == '*';
            }
            return emit(lexer, COMMENT);
        }
        return scan_text(lexer);
    }

    if (!is_ident_start(c)) {
        advance(lexer);
        return scan_text(lexer);
    }

    char word[WORD_SIZE];
    unsigned length = scan_word(lexer, word);
    if (length < WORD_SIZE) {
        if (valid_symbols[PACKAGE] && strcmp(word, "package") == 0) {
            return emit(lexer, PACKAGE);
        }
        if (valid_symbols[IMPORT] && strcmp(word, "import") == 0) {
            return emit(lexer, IMPORT);
        }
        if (valid_symbols[PROPS] && strcmp(word, "props") == 0) {
            return emit(lexer, PROPS);
        }
        if (valid_symbols[SIGNALS] && strcmp(word, "signals") == 0) {
            return emit(lexer, SIGNALS);
        }
        if (valid_symbols[ELSE] && strcmp(word, "else") == 0) {
            return emit(lexer, ELSE);
        }
    }
    if (line_start && length < WORD_SIZE) {
        enum TokenType control = TEXT;
        if (valid_symbols[IF] && strcmp(word, "if") == 0) {
            control = IF;
        } else if (valid_symbols[FOR] && strcmp(word, "for") == 0) {
            control = FOR;
        } else if (valid_symbols[SWITCH] && strcmp(word, "switch") == 0) {
            control = SWITCH;
        }
        if (control != TEXT) {
            // The keyword starts a control line only when the line ends
            // with {. The end of the line is past the end of the token,
            // so a keyword that is text is a text token of its own.
            lexer->mark_end(lexer);
            lexer->result_symbol = line_ends_with_brace(lexer) ? control : TEXT;
            return true;
        }
        if (valid_symbols[CASE] && strcmp(word, "case") == 0 && (lexer->lookahead == ' ' || lexer->lookahead == '\t')) {
            return emit(lexer, CASE);
        }
        if (valid_symbols[DEFAULT] && strcmp(word, "default") == 0 && lexer->lookahead == ':') {
            return emit(lexer, DEFAULT);
        }
    }
    if (line_start && valid_symbols[STATEMENT]) {
        lexer->mark_end(lexer);
        if (scan_statement(lexer)) {
            return true;
        }
    }
    return scan_text(lexer);
}

// scan_keyword reads one of the keywords that follow `else` or open a
// switch body. No text can start at these places.
static bool scan_keyword(TSLexer *lexer, const bool *valid_symbols) {
    if (lexer->lookahead == '}' && valid_symbols[BLOCK_CLOSE]) {
        advance(lexer);
        return emit(lexer, BLOCK_CLOSE);
    }
    if (!is_ident_start(lexer->lookahead)) {
        return false;
    }
    char word[WORD_SIZE];
    unsigned length = scan_word(lexer, word);
    if (length >= WORD_SIZE) {
        return false;
    }
    if (valid_symbols[IF] && strcmp(word, "if") == 0) {
        return emit(lexer, IF);
    }
    if (valid_symbols[CASE] && strcmp(word, "case") == 0) {
        return emit(lexer, CASE);
    }
    if (valid_symbols[DEFAULT] && strcmp(word, "default") == 0 && lexer->lookahead == ':') {
        return emit(lexer, DEFAULT);
    }
    return false;
}

static bool scan(Scanner *scanner, TSLexer *lexer, const bool *valid_symbols) {
    // Text is valid wherever a body node can start, and nowhere else. In
    // error recovery every symbol is valid; the content rules then apply.
    if (valid_symbols[TEXT]) {
        return scan_content(scanner, lexer, valid_symbols);
    }
    if (valid_symbols[RAW_TEXT]) {
        return scan_raw_text(scanner, lexer);
    }
    if (valid_symbols[GO_CODE]) {
        return scan_go_code(lexer);
    }
    if (valid_symbols[CONTROL_HEADER]) {
        return scan_control_header(lexer);
    }
    if (valid_symbols[START_TAG_NAME]) {
        return scan_start_tag_name(scanner, lexer);
    }
    if (valid_symbols[END_TAG_NAME]) {
        return scan_end_tag_name(scanner, lexer);
    }
    if (valid_symbols[FRAGMENT_PARAMETERS] && lexer->lookahead == '(') {
        return scan_fragment_parameters(lexer);
    }

    while (is_space(lexer->lookahead)) {
        skip(lexer);
    }

    if (valid_symbols[IF] || valid_symbols[CASE] || valid_symbols[DEFAULT] || valid_symbols[BLOCK_CLOSE]) {
        return scan_keyword(lexer, valid_symbols);
    }

    switch (lexer->lookahead) {
        case '<':
            advance(lexer);
            if (lexer->lookahead != '!') {
                return false;
            }
            advance(lexer);
            if (lexer->lookahead != '-') {
                return false;
            }
            advance(lexer);
            if (lexer->lookahead != '-') {
                return false;
            }
            advance(lexer);
            return scan_html_comment(lexer);

        case '/':
            if (valid_symbols[SELF_CLOSING_TAG_DELIMITER]) {
                return scan_self_closing_tag_delimiter(scanner, lexer);
            }
            break;
    }

    return false;
}

void *tree_sitter_gx_external_scanner_create() {
    Scanner *scanner = (Scanner *)ts_calloc(1, sizeof(Scanner));
    return scanner;
}

bool tree_sitter_gx_external_scanner_scan(void *payload, TSLexer *lexer, const bool *valid_symbols) {
    Scanner *scanner = (Scanner *)payload;
    return scan(scanner, lexer, valid_symbols);
}

unsigned tree_sitter_gx_external_scanner_serialize(void *payload, char *buffer) {
    Scanner *scanner = (Scanner *)payload;
    return serialize(scanner, buffer);
}

void tree_sitter_gx_external_scanner_deserialize(void *payload, const char *buffer, unsigned length) {
    Scanner *scanner = (Scanner *)payload;
    deserialize(scanner, buffer, length);
}

void tree_sitter_gx_external_scanner_destroy(void *payload) {
    Scanner *scanner = (Scanner *)payload;
    clear_tags(scanner);
    array_delete(&scanner->tags);
    ts_free(scanner);
}
