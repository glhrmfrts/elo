// token types and representations

package parse

type token int

const (
  TOKEN_EOS = iota
  TOKEN_NIL
  TOKEN_TRUE
  TOKEN_FALSE
  TOKEN_IF
  TOKEN_ELSE
  TOKEN_FOR
  TOKEN_FUNC
  TOKEN_ID
  TOKEN_STRING
  TOKEN_NUMBER
  TOKEN_PLUS
  TOKEN_MINUS
  TOKEN_MULT
  TOKEN_DIV
  TOKEN_LT
  TOKEN_LTEQ
  TOKEN_LTLT
  TOKEN_GT
  TOKEN_GTEQ
  TOKEN_GTGT
  TOKEN_EQ
  TOKEN_EQEQ
  TOKEN_COLON
  TOKEN_COMMA
  TOKEN_DOT
  TOKEN_LPAREN
  TOKEN_RPAREN
  TOKEN_LBRACK
  TOKEN_RBRACK
  TOKEN_LBRACE
  TOKEN_RBRACE
  TOKEN_ILLEGAL
)

var keywords = map[string]token{
  "nil": TOKEN_NIL,
  "true": TOKEN_TRUE,
  "false": TOKEN_FALSE,
  "if": TOKEN_IF,
  "else": TOKEN_ELSE,
  "for": TOKEN_FOR,
  "func": TOKEN_FUNC,
}