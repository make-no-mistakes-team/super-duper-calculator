# Calculation Engine

> Scope: Required scientific core, with explicitly optional operation extensions.

## Responsibility

The Go engine parses a bounded expression string, determines its meaning,
evaluates it, and returns a finite result or a structured mathematical error.
It is independently testable without HTTP, a database, or a browser.

It does not own persistence, identity, localization, achievements, or presentation.

## Required language

Expressions use the same syntax in Russian and English interfaces:

- ASCII digits, decimal point `.`, and scientific notation such as `1.25e-3`;
- literals such as `12`, `12.5`, `.5`, and `5.`;
- operators `+`, `-`, `*`, `/`, and `^`;
- parentheses and unary `+` and `-`;
- constants `pi` and `e`;
- Latin-letter function and constant names are case-insensitive (`sin`, `SIN`,
  and `Sin` name the same function); their canonical spelling is lowercase;
- comma-separated function arguments;
- spaces, tabs, and line breaks between tokens, never within a number or name.

| Function | Meaning and domain |
|---|---|
| `sqrt(x)` | Real square root; `x >= 0` |
| `abs(x)` | Absolute value |
| `exp(x)` | Natural exponential |
| `ln(x)` | Natural logarithm; `x > 0` |
| `log(x)` | Base-ten logarithm; `x > 0` |
| `log(x, b)` | Logarithm to base `b`; `x > 0`, `b > 0`, `b != 1` |
| `sin(x)`, `cos(x)`, `tan(x)` | Trigonometry in the selected angle unit |
| `asin(x)`, `acos(x)` | Inverse trigonometry; `-1 <= x <= 1` |
| `atan(x)` | Inverse tangent |

Inverse trigonometric results use the selected angle unit. `pi` always denotes
the same numeric constant; changing angle units does not redefine it.

There is no implicit multiplication: use `2*pi` and `2*(3+4)`, not `2pi` or
`2(3+4)`. No assignment, variables, strings, property access, or arbitrary calls
are accepted. UI buttons may display mathematical symbols but insert canonical
syntax. Decimal commas and translated function names are not alternate grammars.

## Precedence and associativity

From strongest to weakest:

1. Parenthesized expressions and function calls.
2. Optional postfix factorial or percentage.
3. Exponentiation, right-associative.
4. Unary signs.
5. Multiplication and division, left-associative.
6. Addition and subtraction, left-associative.

The right operand of exponentiation may carry a unary sign:

| Expression | Result |
|---|---|
| `2+3*4` | `14` |
| `(2+3)*4` | `20` |
| `8/4*2` | `4` |
| `2^3^2` | `512` |
| `-2^2` | `-4` |
| `(-2)^2` | `4` |
| `2^-3` | `0.125` |

Accepted expressions must consume the entire input.

## Calculation context

- `angleUnit`: `deg` or `rad`; first-use default is `deg`.
- `semanticsVersion`: a service-owned identifier for the mathematical contract.

The effective context is stored with every calculation. UI language, theme,
room membership, and effect preferences are not mathematical context.

## Numerical model

Use real IEEE 754 binary64 arithmetic.

- Literals and operations round according to this model.
- Subnormal values are supported; underflow may round to zero.
- Overflow, `NaN`, and infinite results are errors, not successful answers.
- Division by either positive or negative zero is an error.
- Normalize negative zero to `0` at the application boundary.
- `0^0` is `1`, consistent with the selected numerical exponentiation convention.
- Zero to a negative power is a division-by-zero error.
- A negative base with a non-integer exponent is outside the real-valued domain.
- `NaN` and `Infinity` are not accepted literals or constants.

Tangent at odd multiples of 90 degrees, and corresponding radian inputs such
as `pi/2`, must report a domain error. This includes cases where a library's
approximation of `pi` produces a finite value. Numerical tolerance must not
round small valid results to zero throughout the engine.

### Values and formatting

The canonical value is the shortest locale-independent decimal string that
round-trips to the result's binary64 value. Scientific notation is allowed.
It is the value used for persistence and full-value copying.

The main UI may show up to 12 significant digits for readability. When this
changes the represented value, the display must indicate approximation and make
the full value accessible. Display rounding must never feed subsequent
evaluation or change achievement conditions.

For example, binary64 evaluation of `0.1+0.2` may have the canonical value
`0.30000000000000004`, while the UI presents an explicitly rounded `0.3`.

## First optional extensions

Each extension is advertised only when fully implemented.

| Extension | Syntax and behavior |
|---|---|
| Factorial | `n!`, for integer `n` from `0` through `170`; `0! = 1` |
| Percentage | Postfix `x%`, meaning exactly `x/100` in an expression |
| Remainder | `mod(a, b)`, floating-point remainder with quotient truncated toward zero; `b != 0` |

Factorial above `170` produces numerical overflow; negative or non-integer
arguments produce a domain error.

The postfix percentage operator divides its operand by 100:
`200+10%` means `200+0.1`; a ten-percent increase is `200*(1+10%)`.
The latter is mathematically `220`, but binary64 evaluation can yield
`220.00000000000003`, presented as an explicitly rounded `220`.

Remainder follows the dividend's sign: `mod(-7,3) = -1`. It is not Euclidean
modulo and does not share the `%` spelling.

Only one postfix operator may follow a primary expression without additional
parentheses. `3!!` is rejected, not interpreted as double factorial or as two
factorials. `(3!)!` is explicit repeated factorial. Consequently,
`2^3! = 64` and `-3! = -6`.

## Errors and limits

Mathematical failures use stable, untranslated codes:

- `SYNTAX_ERROR`;
- `UNKNOWN_IDENTIFIER`;
- `UNSUPPORTED_FEATURE`;
- `WRONG_ARITY`;
- `DIVISION_BY_ZERO`;
- `DOMAIN_ERROR`;
- `NUMERIC_OVERFLOW`.

An error includes its stage (`parse` or `evaluate`), safe message parameters,
and a relevant source span where available. Spans are zero-based, half-open
UTF-16 code-unit offsets into the original submitted string, matching browser
string indexing. A missing operand at end of input may use a zero-width span.

Error text is localized outside the engine. Programmer failures are service
errors, not syntax errors attributed to the user.

Initial input budgets are 1,024 UTF-16 code units, 256 tokens, and 32 levels of
syntactic nesting. Exceeding a budget is `EXPRESSION_LIMIT`, handled as a rejected
request by the service. Tokenization, parsing, evaluation, and trace generation
must all respect bounded work. Limits may be tuned together with the advertised
capabilities and verification corpus.

## Facts for other features

The engine can report facts obtained from its actual interpretation:

- operators and named functions used;
- number of operations;
- syntactic nesting depth;
- success value or mathematical error code.

These facts support statistics and humor. Other layers must not infer division,
function usage, or a result by searching arbitrary input text.

## Optional reduction data

If playback is enabled, the service can request an ordered reduction sequence
for the stored expression and context.

- Each step evaluates one reducible operator or function application.
- Independent subexpressions use deterministic left-to-right postorder.
- The active span identifies the redex in that step's displayed expression,
  not an obsolete offset from the original input.
- Replacement preserves the meaning of the remaining expression, including
  necessary parentheses around negative values.
- Intermediate values use canonical values, not rounded UI approximations.
- The original expression remains unchanged.
- A successful sequence ends in the same canonical value as ordinary evaluation.

Atomic signed numbers require no reduction step. Reduction steps exclude
formatting-only changes. Display syntax and evaluation errors in the normal
error interface.

The representation crossing the service boundary is defined in
[Application Service](application-service.md), and playback behavior in
[Visualization & Animation](visualization-and-animation.md).

## Acceptance

The core handles the precedence examples above, `sqrt(81)+2^3 = 17`,
`log(8,2) = 3`, degree/radian contexts, malformed expressions, function domains,
zero division, overflow, and bounded pathological input.

Check transcendental results with appropriate numerical tolerance. Enabled
extensions follow the baseline error, history, and presentation rules.
