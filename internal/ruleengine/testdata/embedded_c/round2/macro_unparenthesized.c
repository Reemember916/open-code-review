/* positive: single-line #define with arg not (x) should fire as hint */
#define SQUARE(x) x * x          /* line 2: should hit (x is bare) */