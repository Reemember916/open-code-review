/* positive: while(1) with no watchdog/break/return should hit require_watchdog_or_exit */
void TaskLoop(void) {
    while (1) {            /* line 3: should hit */
        PollSensors();     /* only this, no exit */
    }
}