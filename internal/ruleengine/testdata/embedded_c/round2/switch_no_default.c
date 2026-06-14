/* positive: switch (...) { should fire as a hint */
void HandleEvent(int evt) {
    switch (evt) {     /* line 3: should hit */
        case 1: doA(); break;
        case 2: doB(); break;
    }
}