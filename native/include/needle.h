#ifndef NEEDLE_H
#define NEEDLE_H

#include <stdint.h>

/* ABI verified against source/needle/needle/__init__.py at v2.0.11. */
int needle_init(const char *system, const char *tools_json, const char *tool_index_path);
int needle_complete(const char *text, int max_new_tokens, char *output_buffer, int output_buffer_size);
void needle_reset(void);
int needle_load(const char *weights_blob, uint64_t weights_size);

#endif
