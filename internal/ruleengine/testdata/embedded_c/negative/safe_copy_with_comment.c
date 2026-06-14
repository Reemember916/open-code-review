/* negative: strcpy in a comment should NOT hit (regex no comment awareness, but
   we use it to assert the rule's behavior is "match anywhere" so this is a known
   weak spot. Test will document the limitation rather than assert a hit.) */
#include <string.h>

void safe_copy(char *dst, const char *src, size_t n) {
    /* TODO: avoid strcpy in legacy code: strcpy(dst, src); */
    if (n > 0) dst[0] = '\0';
}