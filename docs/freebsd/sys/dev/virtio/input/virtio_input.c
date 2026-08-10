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

#include <dev/evdev/input.h>
#include <dev/evdev/evdev.h>
#include <dev/evdev/evdev_private.h>

#include <dev/virtio/input/virtio_input.h>

static int	vtinput_modevent(module_t, int, void *);

static int	vtinput_probe(device_t);
static int	vtinput_attach(device_t);
static int	vtinput_detach(device_t);
static void	vtinput_intr(void *);
static void	vtinput_read_config(struct vtinput_softc *);

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

        vtinput_kbd_driver_load(mod, type, unused);

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
	struct vtinput_softc *sc;
        struct vq_alloc_info vq_info[2];
        int error;

        sc = device_get_softc(dev);
        sc->vtinput_dev = dev;

        vtinput_read_config(sc);

        VQ_ALLOC_INFO_INIT(&vq_info[0], 0, vtinput_intr, sc,
                           &sc->vtinput_event_vq, "%s eventq",
                           device_get_nameunit(dev));
        VQ_ALLOC_INFO_INIT(&vq_info[1], 0, NULL, sc,
                           &sc->vtinput_status_vq, "%s statusq",
                           device_get_nameunit(dev));

        error = virtio_alloc_virtqueues(dev, 2, vq_info);
        if (error != 0) {
                device_printf(dev, "cannot allocate virtqueues: %d\n", error);
                goto fail;
        }

        for (int i = 0; i < 32; i++) {
                struct sglist sg;
                struct sglist_seg segs[1];
                struct virtio_input_event *ev;

                ev = malloc(sizeof(*ev), M_DEVBUF, M_WAITOK);

                sglist_init(&sg, 1, segs);
                sglist_append(&sg, ev, sizeof(*ev));

                virtqueue_enqueue(sc->vtinput_event_vq, ev, &sg, 0, 1);
        }

        virtqueue_notify(sc->vtinput_event_vq);
        virtqueue_enable_intr(sc->vtinput_event_vq);

        virtio_setup_intr(dev, INTR_TYPE_TTY);

        /*
         * Setup evdev devices for keyboard and mouse.
         */

        sc->kbd = evdev_alloc();

        evdev_set_name(sc->kbd, "VirtIO keyboard");
        evdev_set_phys(sc->kbd, device_get_nameunit(dev));
        evdev_set_id(sc->kbd, BUS_VIRTUAL, 1, 1, 1);

        evdev_support_event(sc->kbd, EV_KEY);
        evdev_support_event(sc->kbd, EV_SYN);

        for (int code = 0; code < 256; code++)
                evdev_support_key(sc->kbd, code);

        evdev_register(sc->kbd);

        sc->ptr = evdev_alloc();

        evdev_set_name(sc->ptr, "VirtIO mouse");
        evdev_set_phys(sc->ptr, device_get_nameunit(dev));
        evdev_set_id(sc->ptr, BUS_VIRTUAL, 1, 2, 1);

        evdev_support_event(sc->ptr, EV_KEY);
        evdev_support_event(sc->ptr, EV_ABS);
        evdev_support_event(sc->ptr, EV_SYN);

        evdev_support_key(sc->ptr, BTN_LEFT);
        evdev_support_key(sc->ptr, BTN_RIGHT);
        evdev_support_key(sc->ptr, BTN_MIDDLE);

        evdev_support_abs(sc->ptr, ABS_X, 0, 1024, 0, 0, 0);
        evdev_support_abs(sc->ptr, ABS_Y, 0, 768, 0, 0, 0);

        evdev_register(sc->ptr);

        /*
         * Register keyboad.
         */
        error = vtinput_kbd_driver_attach(dev);
        if (error != 0)
                goto fail;

        device_printf(dev, "vt keyboard registered\n");

        device_printf(dev, "VirtIO Input Device attached\n");

fail:
        if (error != 0)
                vtinput_detach(dev);

        return (error);
}

static int
vtinput_detach(device_t dev)
{
        vtinput_kbd_driver_detach(dev);
        device_printf(dev, "VirtIO Input Device detached\n");
        return (0);
}

static void
vtinput_intr(void *arg)
{
        struct vtinput_softc *sc = arg;
        struct virtqueue *vq = sc->vtinput_event_vq;
        struct virtio_input_event *ev;
        int len;

        while ((ev = virtqueue_dequeue(vq, &len)) != NULL) {
                struct sglist sg;
                struct sglist_seg segs[1];

                if (ev->type == EV_KEY && ev->code < 256) {
                        evdev_push_event(sc->kbd, ev->type, ev->code,
                                         ev->value);

                        uint8_t scancode = ev->code & 0x7F;

                        mtx_lock(&sc->vtinput_mtx);

                        if (ev->value == 0) {
                                scancode |= 0x80; /* Break code */
                        }

                        if (sc->vtinput_fifo_count < VTINPUT_FIFOSZ) {
                                sc->vtinput_fifo[sc->vtinput_fifo_tail] = scancode;
                                sc->vtinput_fifo_tail = (sc->vtinput_fifo_tail + 1) % VTINPUT_FIFOSZ;
                                sc->vtinput_fifo_count++;

#if 0
                                device_printf(sc->vtinput_dev, "INTR: pushed scancode 0x%02X (ev->code=%d, val=%d)\n",
                                              scancode, ev->code, ev->value);
#endif
                        }

                        mtx_unlock(&sc->vtinput_mtx);

                        vtinput_kbd_intr(&sc->vt_kbd, NULL);
                }
                else if (ev->type == EV_KEY && ev->code >= BTN_MOUSE) {
                        evdev_push_event(sc->ptr, ev->type, ev->code,
                                         ev->value);
                }
                else if (ev->type == EV_ABS) {
                        evdev_push_event(sc->ptr, ev->type, ev->code,
                                         ev->value);
                }
                if (ev->type == EV_SYN) {
                        evdev_sync(sc->kbd);
                        evdev_sync(sc->ptr);
                }

                /* Enqueue event. */
                sglist_init(&sg, 1, segs);
                sglist_append(&sg, ev, sizeof(*ev));

                virtqueue_enqueue(vq, ev, &sg, 0, 1);
        }

        virtqueue_notify(vq);
        virtqueue_enable_intr(sc->vtinput_event_vq);
}

static void
vtinput_read_config(struct vtinput_softc *sc)
{
        device_t dev;
        struct virtio_input_config inputcfg;

        dev = sc->vtinput_dev;

        bzero(&inputcfg, sizeof(inputcfg));

#define VTINPUT_SET_CONFIG(_dev, _field, _cfg)                  \
        virtio_write_device_config(_dev,                        \
            offsetof(struct virtio_input_config, _field),       \
            &(_cfg)->_field, sizeof((_cfg)->_field))

#define VTINPUT_GET_CONFIG(_dev, _field, _cfg)                  \
        virtio_read_device_config(_dev,                         \
            offsetof(struct virtio_input_config, _field),       \
            &(_cfg)->_field, sizeof((_cfg)->_field))

        inputcfg.select = VIRTIO_INPUT_CFG_ID_NAME;
        VTINPUT_SET_CONFIG(dev, select, &inputcfg);
        VTINPUT_GET_CONFIG(dev, size, &inputcfg);

        if (inputcfg.size > 127)
                inputcfg.size = 127;

        for (uint8_t i = 0; i < inputcfg.size; i++) {
                virtio_read_device_config(dev, 8+i, &inputcfg.u.string[i], 1);
        }
        device_printf(dev, "%s\n", inputcfg.u.string);
}

/*
 * Local Variables:
 * c-file-style: "bsd"
 * End:
 */
