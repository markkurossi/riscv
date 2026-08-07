/*-
 * SPDX-License-Identifier: BSD-2-Clause
 *
 * Copyright (c) 2026, Markku Rossi <mtr@iki.fi>
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice unmodified, this list of conditions, and the following
 *    disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS OR
 * IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES
 * OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT,
 * INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
 * NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
 * THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

/* Driver for VirtIO input device. */

#include <sys/param.h>
#include <sys/types.h>
#include <sys/bus.h>
#include <sys/callout.h>
#include <sys/fbio.h>
#include <sys/kernel.h>
#include <sys/malloc.h>
#include <sys/module.h>
#include <sys/sglist.h>

#include <machine/atomic.h>
#include <machine/bus.h>
#include <machine/resource.h>

#include <vm/vm.h>
#include <vm/pmap.h>

#include <dev/virtio/virtio.h>
#include <dev/virtio/virtqueue.h>
#include <dev/virtio/input/virtio_input.h>

struct vtinput_softc {
        struct fb_info vtinput_fb_info;
        device_t       vtinput_dev;
}

static int	vtinput_modevent(module_t, int, void *);

static int	vtinput_probe(device_t);
static int	vtinput_attach(device_t);
static int	vtinput_detach(device_t);

static device_method_t vtinput_methods[] = {
        /* Device methods. */
	DEVMETHOD(device_probe,		vtinput_probe),
	DEVMETHOD(device_attach,	vtinput_attach),
	DEVMETHOD(device_detach,	vtinput_detach),

	DEVMETHOD_END
};

static driver_t vtinput_driver = {
	"vtinput",
	vtinput_methods,
	sizeof(struct vtinput_softc)
};

VIRTIO_DRIVER_MODULE(virtio_input, vtinput_driver, vtinput_modevent, NULL);
MODULE_VERSION(virtio_input, 1);
MODULE_DEPEND(virtio_input, virtio, 1, 1, 1);

VIRTIO_SIMPLE_PNPINFO(virtio_input, VIRTIO_ID_INPUT,
    "VirtIO Input Device");

static int
vtinput_modevent(module_t mod, int type, void *unused)
{
	int error;

	switch (type) {
	case MOD_LOAD:
	case MOD_QUIESCE:
	case MOD_UNLOAD:
	case MOD_SHUTDOWN:
		error = 0;
		break;
	default:
		error = EOPNOTSUPP;
		break;
	}

	return (error);
}

static int
vtinput_probe(device_t dev)
{
	return (VIRTIO_SIMPLE_PROBE(dev, virtio_input));
}

static int
vtinput_attach(device_t dev)
{
	struct vtgpu_softc *sc;

        sc = device_get_softc(dev);
        sc->vtinput_dev = dev;

        device_printf(dev, "VirtIO Input Device attached\n");

        return (0);
}

static int
vtinput_detach(device_t dev)
{
        device_printf(dev, "VirtIO Input Device detached\n");
        return (0);
}
/*
 * Local Variables:
 * c-file-style: "bsd"
 * End:
 */
