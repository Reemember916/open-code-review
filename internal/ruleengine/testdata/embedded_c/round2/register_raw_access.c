/* positive: volatile pointer cast to address should hit use_access_macro */
void InitGpio(void) {
    *(volatile uint32_t *)0x400FF000 = 0x01;  /* line 3: should hit */
}