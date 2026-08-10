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

#include <dev/evdev/input.h>
#include <dev/evdev/evdev.h>
#include <dev/evdev/evdev_private.h>
#include <dev/kbd/kbdreg.h>
#include <dev/kbd/kbdtables.h>
#include <sys/kbio.h>
#include <sys/mutex.h>

#define VTINPUT_FIFOSZ 128

struct vtinput_softc {
        struct fb_info		vtinput_fb_info;
        device_t		vtinput_dev;

        struct virtqueue       *vtinput_event_vq;
        struct virtqueue       *vtinput_status_vq;

        struct evdev_dev       *kbd;
        struct evdev_dev       *ptr;

        keyboard_t 	       *vt_kbd;

        /* Keyboard FIFO and synchronization */
        struct mtx              vtinput_mtx;
        uint8_t                 vtinput_fifo[VTINPUT_FIFOSZ];
        u_int                   vtinput_fifo_head;
        u_int                   vtinput_fifo_tail;
        u_int                   vtinput_fifo_count;

        /* State machine tracking */
        int                     vtinput_prefix;
        int                     vtinput_kbd_state;
        int                     vtinput_kbd_accents;
        int                     vtinput_kbd_mode;
};

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

static int	         vtinput_kbd_probe(int, void *, int);
static int	       	 vtinput_kbd_init(int, keyboard_t **, void *, int);
static kbd_term_t	 vtinput_kbd_term;
static kbd_intr_t	 vtinput_kbd_intr;
static kbd_test_if_t	 vtinput_kbd_test_if;
static int		 vtinput_kbd_enable(keyboard_t *);
static int		 vtinput_kbd_disable(keyboard_t *);
static int		 vtinput_kbd_read(keyboard_t *, int);
static int		 vtinput_kbd_check(keyboard_t *);
static kbd_read_char_t	 vtinput_kbd_read_char;
static kbd_check_char_t	 vtinput_kbd_check_char;
static kbd_ioctl_t	 vtinput_kbd_ioctl;
static kbd_lock_t	 vtinput_kbd_lock;
static kbd_clear_state_t vtinput_kbd_clear_state;
static kbd_get_state_t	 vtinput_kbd_get_state;
static kbd_set_state_t	 vtinput_kbd_set_state;
static kbd_poll_mode_t	 vtinput_kbd_poll;

static keyboard_switch_t vtinput_kbd_sw = {
        .probe = 	vtinput_kbd_probe,
        .init  = 	vtinput_kbd_init,
        .term = 	vtinput_kbd_term,
        .intr = 	vtinput_kbd_intr,
        .test_if = 	vtinput_kbd_test_if,
        .enable = 	vtinput_kbd_enable,
        .disable = 	vtinput_kbd_disable,
        .read = 	vtinput_kbd_read,
        .check = 	vtinput_kbd_check,
	.read_char =	vtinput_kbd_read_char,
	.check_char =	vtinput_kbd_check_char,
	.ioctl =	vtinput_kbd_ioctl,
	.lock =		vtinput_kbd_lock,
	.clear_state =	vtinput_kbd_clear_state,
	.get_state =	vtinput_kbd_get_state,
	.set_state =	vtinput_kbd_set_state,
	.poll =		vtinput_kbd_poll,
};

KEYBOARD_DRIVER(vtinputkbd, vtinput_kbd_sw, NULL);
MODULE_DEPEND(virtio_input, kbd, 1, 1, 1);

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

        evdev_set_name(sc->kbd, "virtio-keyboard");
        evdev_set_phys(sc->kbd, device_get_nameunit(dev));
        evdev_set_id(sc->kbd, BUS_VIRTUAL, 1, 1, 1);

        evdev_support_event(sc->kbd, EV_KEY);
        evdev_support_event(sc->kbd, EV_SYN);

        for (int code = 0; code < 256; code++)
                evdev_support_key(sc->kbd, code);

        evdev_register(sc->kbd);

        sc->ptr = evdev_alloc();

        evdev_set_name(sc->ptr, "virtio-mouse");
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

        mtx_init(&sc->vtinput_mtx, "vtinput", NULL, MTX_DEF|MTX_RECURSE);

        /* Bulletproof default mode initialization */
        sc->vtinput_kbd_mode = K_XLATE;

        keyboard_switch_t *sw = kbd_get_switch("vtinputkbd");
        if (sw == NULL) {
                device_printf(dev, "vtinputkbd switch NOT found\n");
                goto fail;
        }

        error = (*sw->probe)(device_get_unit(dev), sc, 0);
        if (error != 0) {
                device_printf(dev, "vtinputkbd probe failed: %d\n", error);
                goto fail;
        }

        sc->vt_kbd = NULL;
        error = (*sw->init)(device_get_unit(dev), &sc->vt_kbd, sc, 0);
        if (error != 0) {
                device_printf(dev, "vtinputkbd init failed: %d\n", error);
                goto fail;
        }

        (*sw->enable)(sc->vt_kbd);

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
	struct vtinput_softc *sc;

        sc = device_get_softc(dev);

        mtx_destroy(&sc->vtinput_mtx);

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

                                device_printf(sc->vtinput_dev, "INTR: pushed scancode 0x%02X (ev->code=%d, val=%d)\n",
                                              scancode, ev->code, ev->value);
                        }

                        mtx_unlock(&sc->vtinput_mtx);

                        vtinput_kbd_intr(sc->vt_kbd, NULL);
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

static int
vtinput_kbd_probe(int unit, void *arg, int flags)
{
        return (0);
}

static int
vtinput_kbd_init(int unit, keyboard_t **kbdp, void *arg, int flags)
{
        keyboard_t *kbd;
        keymap_t *keymap;
        accentmap_t *accmap;
        fkeytab_t *fkeymap;
        int fkeymap_size;

        if (*kbdp == NULL) {
                kbd = malloc(sizeof(*kbd), M_DEVBUF, M_WAITOK | M_ZERO);
                keymap = malloc(sizeof(key_map), M_DEVBUF, M_WAITOK);
                accmap = malloc(sizeof(accent_map), M_DEVBUF, M_WAITOK);
                fkeymap = malloc(sizeof(fkey_tab), M_DEVBUF, M_WAITOK);
                fkeymap_size = sizeof(fkey_tab) / sizeof(fkey_tab[0]);
                *kbdp = kbd;
        } else {
                kbd = *kbdp;
                keymap = kbd->kb_keymap;
                accmap = kbd->kb_accentmap;
                fkeymap = kbd->kb_fkeytab;
                fkeymap_size = kbd->kb_fkeytab_size;
        }

        if (!KBD_IS_PROBED(kbd)) {
                kbd_init_struct(kbd, "vtinputkbd", KB_101, unit, flags, 0, 0);

                bcopy(&key_map, keymap, sizeof(key_map));
                bcopy(&accent_map, accmap, sizeof(accent_map));
                bcopy(fkey_tab, fkeymap,
                    imin(fkeymap_size * sizeof(fkeymap[0]), sizeof(fkey_tab)));
                kbd_set_maps(kbd, keymap, accmap, fkeymap, fkeymap_size);
                kbd->kb_data = arg;  /* your vtinput_softc *sc */
                KBD_FOUND_DEVICE(kbd);
                KBD_PROBE_DONE(kbd);
        }

        if (!KBD_IS_INITIALIZED(kbd) && !(flags & KB_CONF_PROBE_ONLY)) {
                kbd->kb_config = flags & ~KB_CONF_PROBE_ONLY;
                KBD_INIT_DONE(kbd);
        }

        if (!KBD_IS_CONFIGURED(kbd)) {
                if (kbd_register(kbd) < 0)
                        return (ENXIO);
                KBD_CONFIG_DONE(kbd);
        }

        return (0);
}

/* Finish using this keyboard */
static int
vtinput_kbd_term(keyboard_t *kbd)
{
	kbd_unregister(kbd);

	free(kbd->kb_keymap, M_DEVBUF);
	free(kbd->kb_accentmap, M_DEVBUF);
	free(kbd->kb_fkeytab, M_DEVBUF);
	free(kbd, M_DEVBUF);

	return (0);
}

/* Keyboard interrupt routine */
static int
vtinput_kbd_intr(keyboard_t *kbd, void *arg)
{
	int	c;

	if (KBD_IS_ACTIVE(kbd) && KBD_IS_BUSY(kbd)) {
		/* let the callback function to process the input */
		(*kbd->kb_callback.kc_func)(kbd, KBDIO_KEYINPUT,
					    kbd->kb_callback.kc_arg);
	} else {
		/* read and discard the input; no one is waiting for input */
		do {
			c = vtinput_kbd_read_char(kbd, FALSE);
		} while (c != NOKEY);
	}

	return (0);
}

/* Test the interface to the device */
static int
vtinput_kbd_test_if(keyboard_t *kbd)
{
	return (0);
}

/*
 * Enable the access to the device; until this function is called,
 * the client cannot read from the keyboard.
 */

static int
vtinput_kbd_enable(keyboard_t *kbd)
{
	KBD_ACTIVATE(kbd);
	return (0);
}

/* Disallow the access to the device */
static int
vtinput_kbd_disable(keyboard_t *kbd)
{
	KBD_DEACTIVATE(kbd);
	return (0);
}

static int
vtinput_kbd_read(keyboard_t *kbd, int wait)
{
        return (vtinput_kbd_read_char(kbd, wait));
}

static int
vtinput_kbd_check(keyboard_t *kbd)
{
        return (vtinput_kbd_check_char(kbd));
}

static u_int
vtinput_kbd_read_char(keyboard_t *kbd, int wait)
{
        struct vtinput_softc *sc = kbd->kb_data;
        uint32_t action;
        uint8_t scancode;
        int keycode;

next_code:
        mtx_lock(&sc->vtinput_mtx);
        if (sc->vtinput_fifo_count == 0) {
                mtx_unlock(&sc->vtinput_mtx);
                return (NOKEY);
        }

        /* Pop scancode from ring buffer */
        scancode = sc->vtinput_fifo[sc->vtinput_fifo_head];
        sc->vtinput_fifo_head = (sc->vtinput_fifo_head + 1) % VTINPUT_FIFOSZ;
        sc->vtinput_fifo_count--;
        mtx_unlock(&sc->vtinput_mtx);

        /* DEBUG: Log popped scancode and current console mode */
        device_printf(((struct vtinput_softc *)kbd->kb_data)->vtinput_dev,
                      "READ_CHAR: popped scancode 0x%02X, mode=%d\n",
                      scancode, sc->vtinput_kbd_mode);

        /* K_RAW: The graphical console wants raw scancodes. Return immediately. */
        if (sc->vtinput_kbd_mode == K_RAW)
                return (scancode);

        /* AT-scancode E0/E1 prefix state machine */
        if (sc->vtinput_prefix == 0 && (scancode == 0xE0 || scancode == 0xE1)) {
                sc->vtinput_prefix = scancode;
                goto next_code;
        }

        /* Map prefixed codes to internal extended keycodes */
        keycode = scancode & 0x7F;
        if (sc->vtinput_prefix == 0xE0) {
                keycode |= 0x80; /* Flag extended E0 */
        }

        /* Reset prefix state */
        sc->vtinput_prefix = 0;

        /* K_CODE: The multiplexer or console wants cooked keycodes with make/break flags. */
        if (sc->vtinput_kbd_mode == K_CODE)
                return (keycode | (scancode & 0x80));

        /* K_XLATE: The text console wants translated ASCII/actions directly. */
        action = genkbd_keyaction(kbd, keycode, scancode & 0x80,
                                  &sc->vtinput_kbd_state, &sc->vtinput_kbd_accents);

        if (action == NOKEY) {
                goto next_code;
        }

        return (action);
}

/* Check if char is waiting */
static int
vtinput_kbd_check_char(keyboard_t *kbd)
{
        struct vtinput_softc *sc = kbd->kb_data;
        int ready;

        if (!KBD_IS_ACTIVE(kbd))
                return (FALSE);

        mtx_lock(&sc->vtinput_mtx);
        ready = (sc->vtinput_fifo_count > 0);
        mtx_unlock(&sc->vtinput_mtx);

        return (ready);
}

static int
vtinput_kbd_ioctl(keyboard_t *kbd, u_long cmd, caddr_t arg)
{
        struct vtinput_softc *sc = kbd->kb_data;
        int error;

        switch (cmd) {
        case KDGKBMODE:
                *(int *)arg = sc->vtinput_kbd_mode;
                return (0);
        case KDSKBMODE:
                if (!KBD_IS_ACTIVE(kbd))
                        return (EPERM);
                switch (*(int *)arg) {
                case K_XLATE:
                case K_RAW:
                case K_CODE:
                        sc->vtinput_kbd_mode = *(int *)arg;
                        return (0);
                default:
                        return (EINVAL);
                }
        }

        /* Let the general keyboard layer handle standard ioctls */
        error = genkbd_commonioctl(kbd, cmd, arg);
        if (error == ENOIOCTL) {
                error = ENOTTY;
        }

        return (error);
}

/* Lock the access to the keyboard */
static int
vtinput_kbd_lock(keyboard_t *kbd, int lock)
{
	return (1); /* XXX */
}

static void
vtinput_kbd_clear_state(keyboard_t *kbd)
{
}

/* Save the internal state */
static int
vtinput_kbd_get_state(keyboard_t *kbd, void *buf, size_t len)
{
	return (0);
}

/* Set the internal state */
static int
vtinput_kbd_set_state(keyboard_t *kbd, void *buf, size_t len)
{
	return (0);
}

/* Set polling */
static int
vtinput_kbd_poll(keyboard_t *kbd, int on)
{
	return (0);
}

/*
 * Local Variables:
 * c-file-style: "bsd"
 * End:
 */
