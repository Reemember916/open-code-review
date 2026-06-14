/* positive: ISR calling malloc should hit embedded.isr.no_dynamic_allocation */
#include <stdlib.h>

interrupt void SpiDmaIsr(void) {
    void *p = malloc(64);  /* line 5: should hit */
    use(p);
}