/* positive: should trigger embedded.isr.no_delay_call (call backend, role=isr) */
#include "bsp.h"

interrupt void TimerVectorHandler(void) {
    DelayMs(1);  /* line 5: should hit */
}