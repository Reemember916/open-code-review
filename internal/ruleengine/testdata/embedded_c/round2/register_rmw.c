/* positive: ALL_CAPS register compound assignment should hit no_read_modify_write_on_status_clear */
void ClearStatus(void) {
    CANSTS |= 0x04;   /* line 3: should hit (|= on ALL_CAPS) */
    if (BIT_TEST_OK == state) {
        StateOk();
    }
}
