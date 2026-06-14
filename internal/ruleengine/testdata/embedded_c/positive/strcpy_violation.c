/* positive: should trigger embedded.memory.no_strcpy (regex) */
#include <string.h>

void copy_buffer(char *dst, const char *src) {
    strcpy(dst, src);  /* line 5: should hit */
}