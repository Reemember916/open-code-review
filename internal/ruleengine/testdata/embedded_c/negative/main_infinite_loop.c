/* negative: while(1) in main() should be suppressed by function_name_not_any */
#include <stdint.h>

void main(void) {
    while (1) {           /* line 5: should NOT hit (function_name_not_any=main) */
        /* event loop body */
        break;
    }
}