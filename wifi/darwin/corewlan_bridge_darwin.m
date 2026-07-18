//go:build darwin && cgo

#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>

#include <stdlib.h>
#include <string.h>

#include "corewlan_bridge.h"

static void wifitui_set_error(char **destination, NSString *message) {
    if (destination == NULL || message == nil) {
        return;
    }
    *destination = strdup(message.UTF8String);
}

static NSString *wifitui_security(CWNetwork *network) {
    if ([network supportsSecurity:kCWSecurityNone]) {
        return @"open";
    }
    if ([network supportsSecurity:kCWSecurityWEP] ||
        [network supportsSecurity:kCWSecurityDynamicWEP]) {
        return @"wep";
    }
    if ([network supportsSecurity:kCWSecurityUnknown]) {
        return @"unknown";
    }
    return @"wpa";
}

static NSNumber *wifitui_frequency(CWNetwork *network) {
    CWChannel *channel = network.wlanChannel;
    if (channel == nil) {
        return @0;
    }
    NSInteger number = channel.channelNumber;
    switch (channel.channelBand) {
        case kCWChannelBand2GHz:
            if (number == 14) {
                return @2484;
            }
            if (number >= 1 && number <= 13) {
                return @(2407 + 5 * number);
            }
            break;
        case kCWChannelBand5GHz:
            if (number > 0) {
                return @(5000 + 5 * number);
            }
            break;
#if defined(__MAC_13_0) && __MAC_OS_X_VERSION_MAX_ALLOWED >= __MAC_13_0
        case kCWChannelBand6GHz:
            if (number > 0) {
                return @(5950 + 5 * number);
            }
            break;
#endif
        default:
            break;
    }
    return @0;
}

int wifitui_corewlan_scan(const char *device, char **output, char **error_message) {
    @autoreleasepool {
        if (output != NULL) {
            *output = NULL;
        }
        if (error_message != NULL) {
            *error_message = NULL;
        }

        NSString *device_name = device == NULL ? nil : [NSString stringWithUTF8String:device];
        CWInterface *interface = [[CWWiFiClient sharedWiFiClient] interfaceWithName:device_name];
        if (interface == nil) {
            wifitui_set_error(error_message, [NSString stringWithFormat:@"CoreWLAN interface %@ is unavailable", device_name]);
            return 1;
        }

        NSError *scan_error = nil;
        NSSet<CWNetwork *> *networks = [interface scanForNetworksWithName:nil error:&scan_error];
        if (networks == nil) {
            NSString *message = scan_error == nil
                ? @"CoreWLAN returned neither scan results nor an error"
                : [NSString stringWithFormat:@"CoreWLAN scan failed: %@ (%@:%ld)",
                    scan_error.localizedDescription, scan_error.domain, (long)scan_error.code];
            wifitui_set_error(error_message, message);
            if ([scan_error.domain isEqualToString:CWErrorDomain]) {
                switch ((CWErr)scan_error.code) {
                    case kCWOperationNotPermittedErr:
                        return 4;
                    case kCWTimeoutErr:
                    case kCWSupplicantTimeoutErr:
                        return 5;
                    case kCWNotSupportedErr:
                        return 6;
                    default:
                        break;
                }
            }
            return 2;
        }

        NSMutableArray<NSDictionary *> *serialized = [NSMutableArray arrayWithCapacity:networks.count];
        for (CWNetwork *network in networks) {
            NSString *ssid = network.ssid;
            if (ssid == nil || ssid.length == 0) {
                continue;
            }
            [serialized addObject:@{
                @"ssid": ssid,
                @"bssid": network.bssid ?: @"",
                @"security": wifitui_security(network),
                @"rssi": @(network.rssiValue),
                @"frequency": wifitui_frequency(network),
            }];
        }
        if (networks.count > 0 && serialized.count == 0) {
            wifitui_set_error(error_message,
                @"CoreWLAN returned networks without SSIDs; Location Services permission may be required");
            return 4;
        }

        NSError *json_error = nil;
        NSData *json_data = [NSJSONSerialization dataWithJSONObject:serialized options:0 error:&json_error];
        if (json_data == nil) {
            NSString *message = json_error == nil
                ? @"CoreWLAN results could not be serialized"
                : [NSString stringWithFormat:@"CoreWLAN results could not be serialized: %@",
                    json_error.localizedDescription];
            wifitui_set_error(error_message, message);
            return 3;
        }

        if (output != NULL) {
            NSUInteger length = json_data.length;
            char *json = malloc(length + 1);
            if (json == NULL) {
                wifitui_set_error(error_message, @"CoreWLAN result JSON could not be allocated");
                return 3;
            }
            memcpy(json, json_data.bytes, length);
            json[length] = '\0';
            *output = json;
        }
        return 0;
    }
}

void wifitui_corewlan_free(char *value) {
    free(value);
}
