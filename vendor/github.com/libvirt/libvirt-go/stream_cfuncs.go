/*
 * This file is part of the libvirt-go project
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 * Copyright (c) 2013 Alex Zorin
 * Copyright (C) 2016 Red Hat, Inc.
 *
 */

package libvirt

/*
#cgo pkg-config: libvirt
#include <libvirt/libvirt.h>
#include <libvirt/virterror.h>
#include <stdint.h>
#include <stdlib.h>
#include "stream_cfuncs.h"

int streamSourceCallback(virStreamPtr st, char *cdata, size_t nbytes, int callbackID);
int streamSinkCallback(virStreamPtr st, const char *cdata, size_t nbytes, int callbackID);

static int streamSourceCallbackHelper(virStreamPtr st, char *data, size_t nbytes, void *opaque)
{
    int *callbackID = opaque;

    return streamSourceCallback(st, data, nbytes, *callbackID);
}

static int streamSinkCallbackHelper(virStreamPtr st, const char *data, size_t nbytes, void *opaque)
{
    int *callbackID = opaque;

    return streamSinkCallback(st, data, nbytes, *callbackID);
}

int virStreamSendAll_cgo(virStreamPtr st, int callbackID)
{
    return virStreamSendAll(st, streamSourceCallbackHelper, &callbackID);
}


int virStreamRecvAll_cgo(virStreamPtr st, int callbackID)
{
    return virStreamRecvAll(st, streamSinkCallbackHelper, &callbackID);
}

void streamEventCallback(virStreamPtr st, int events, int callbackID);

static void streamEventCallbackHelper(virStreamPtr st, int events, void *opaque)
{
    streamEventCallback(st, events, (int)(intptr_t)opaque);
}

int virStreamEventAddCallback_cgo(virStreamPtr st, int events, int callbackID)
{
    return virStreamEventAddCallback(st, events, streamEventCallbackHelper, (void *)(intptr_t)callbackID, NULL);
}

*/
import "C"
