/* positive: ISR calling SemaphorePend should hit embedded.isr.no_blocking_call */
#include "rtos.h"

interrupt void AdcIsrHandler(void) {
    SemaphorePend(g_adcSem, 1000);  /* line 5: should hit */
}