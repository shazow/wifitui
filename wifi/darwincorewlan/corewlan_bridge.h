#ifndef WIFITUI_COREWLAN_BRIDGE_H
#define WIFITUI_COREWLAN_BRIDGE_H

// Returns 0 on success, or one of the status values mirrored in
// coreWLANStatusError. The caller owns returned strings and must release them
// with wifitui_corewlan_free.
int wifitui_corewlan_scan(const char *device, char **output, char **error_message);
void wifitui_corewlan_free(char *value);

#endif
