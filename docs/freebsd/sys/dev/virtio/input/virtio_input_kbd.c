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

/* Keyboard driver for VirtIO input device. */

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

#include <dev/kbd/kbdreg.h>
#include <dev/kbd/kbdtables.h>
#include <sys/kbio.h>
#include <sys/mutex.h>

#include <dev/virtio/input/virtio_input.h>

#define VIOKBD_DRIVER_NAME	"vtinputkbd"

static int	         viokbd_probe(int, void *, int);
static int	       	 viokbd_init(int, keyboard_t **, void *, int);
static kbd_term_t	 viokbd_term;
static kbd_intr_t	 viokbd_intr;
static kbd_test_if_t	 viokbd_test_if;
static int		 viokbd_enable(keyboard_t *);
static int		 viokbd_disable(keyboard_t *);
static int		 viokbd_read(keyboard_t *, int);
static int		 viokbd_check(keyboard_t *);
static kbd_read_char_t	 viokbd_read_char;
static kbd_check_char_t	 viokbd_check_char;
static kbd_ioctl_t	 viokbd_ioctl;
static kbd_lock_t	 viokbd_lock;
static kbd_clear_state_t viokbd_clear_state;
static kbd_get_state_t	 viokbd_get_state;
static kbd_set_state_t	 viokbd_set_state;
static kbd_poll_mode_t	 viokbd_poll;

static keyboard_switch_t viokbd_sw = {
        .probe = 	viokbd_probe,
        .init  = 	viokbd_init,
        .term = 	viokbd_term,
        .intr = 	viokbd_intr,
        .test_if = 	viokbd_test_if,
        .enable = 	viokbd_enable,
        .disable = 	viokbd_disable,
        .read = 	viokbd_read,
        .check = 	viokbd_check,
	.read_char =	viokbd_read_char,
	.check_char =	viokbd_check_char,
	.ioctl =	viokbd_ioctl,
	.lock =		viokbd_lock,
	.clear_state =	viokbd_clear_state,
	.get_state =	viokbd_get_state,
	.set_state =	viokbd_set_state,
	.poll =		viokbd_poll,
};

KEYBOARD_DRIVER(vtinputkbd, viokbd_sw, NULL);
MODULE_DEPEND(virtio_input, vtinputkbd, 1, 1, 1);

int
vtinput_kbd_driver_load(module_t mod, int what, void *arg)
{
        switch (what) {
	case MOD_LOAD:
		kbd_add_driver(&vtinputkbd_kbd_driver);
		break;
	case MOD_UNLOAD:
		kbd_delete_driver(&vtinputkbd_kbd_driver);
		break;
        }
        return (0);
}

int
vtinput_kbd_driver_attach(device_t dev)
{
	struct vtinput_softc *sc = device_get_softc(dev);
        int unit = device_get_unit(dev);
        keyboard_t *kbd = &sc->vt_kbd;
        keyboard_switch_t *sw;

        mtx_init(&sc->vtinput_mtx, "vtinput", NULL, MTX_DEF|MTX_RECURSE);

        sw = kbd_get_switch(VIOKBD_DRIVER_NAME);
        if (sw == NULL) {
                device_printf(dev, "kbd_get_switch failed\n");
                return (ENXIO);
        }

        kbd_init_struct(kbd, VIOKBD_DRIVER_NAME, KB_101, unit, 0, 0, 0);
        kbd->kb_data = (void *) sc;
        kbd_set_maps(kbd, &key_map, &accent_map, fkey_tab, nitems(fkey_tab));
        KBD_FOUND_DEVICE(kbd);
        viokbd_clear_state(kbd);
        KBD_PROBE_DONE(kbd);
        KBD_INIT_DONE(kbd);
        sc->viokbd_mode = K_XLATE;
        (sw->enable)(kbd);

        if (kbd_register(kbd) < 0) {
                goto detach;
        }
        KBD_CONFIG_DONE(kbd);
#if 0
        if (kbd_attach(kbd)) {
                goto detach;
        }
#endif
        kbdd_diag(kbd, 1);

        return (0);
detach:
        vtinput_kbd_driver_detach(dev);
        return (ENXIO);
}

int
vtinput_kbd_driver_detach(device_t dev)
{
	int error = 0;
	struct vtinput_softc *sc = device_get_softc(dev);

	viokbd_disable(&sc->vt_kbd);

	if (KBD_IS_CONFIGURED(&sc->vt_kbd)) {
		error = kbd_unregister(&sc->vt_kbd);
		if (error) {
			device_printf(dev, "WARNING: kbd_unregister() "
			    "returned non-zero! (ignored)\n");
		}
	}
	error = kbd_detach(&sc->vt_kbd);

        mtx_destroy(&sc->vtinput_mtx);

	return (error);
}

/* Keyboard interrupt routine */
int
vtinput_kbd_intr(keyboard_t *kbd, void *arg)
{
	int c;

	if (KBD_IS_ACTIVE(kbd) && KBD_IS_BUSY(kbd)) {
		/* let the callback function to process the input */
		(*kbd->kb_callback.kc_func)(kbd, KBDIO_KEYINPUT,
					    kbd->kb_callback.kc_arg);
	} else {
		/* read and discard the input; no one is waiting for input */
		do {
			c = viokbd_read_char(kbd, FALSE);
		} while (c != NOKEY);
	}

	return (0);
}

static int
viokbd_probe(int unit, void *arg, int flags)
{
        return (ENXIO);
}

static int
viokbd_init(int unit, keyboard_t **kbdp, void *arg, int flags)
{
        return (ENXIO);
}

/* Finish using this keyboard */
static int
viokbd_term(keyboard_t *kbd)
{
        return (ENXIO);
}

/* Keyboard interrupt routine */
static int
viokbd_intr(keyboard_t *kbd, void *arg)
{
        return (0);
}

/* Test the interface to the device */
static int
viokbd_test_if(keyboard_t *kbd)
{
	return (0);
}

/*
 * Enable the access to the device; until this function is called,
 * the client cannot read from the keyboard.
 */
static int
viokbd_enable(keyboard_t *kbd)
{
	KBD_ACTIVATE(kbd);
	return (0);
}

/* Disallow the access to the device */
static int
viokbd_disable(keyboard_t *kbd)
{
	KBD_DEACTIVATE(kbd);
	return (0);
}

static int
viokbd_read(keyboard_t *kbd, int wait)
{
        return (viokbd_read_char(kbd, wait));
}

static int
viokbd_check(keyboard_t *kbd)
{
        return (viokbd_check_char(kbd));
}

static u_int
viokbd_read_char(keyboard_t *kbd, int wait)
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

#if 0
        /* DEBUG: Log popped scancode and current console mode */
        device_printf(((struct vtinput_softc *)kbd->kb_data)->vtinput_dev,
                      "READ_CHAR: popped scancode 0x%02X, mode=%d\n",
                      scancode, sc->viokbd_mode);
#endif

        /* K_RAW: The graphical console wants raw scancodes. Return immediately. */
        if (sc->viokbd_mode == K_RAW)
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
        if (sc->viokbd_mode == K_CODE)
                return (keycode | (scancode & 0x80));

        /* K_XLATE: The text console wants translated ASCII/actions directly. */
        action = genkbd_keyaction(kbd, keycode, scancode & 0x80,
                                  &sc->viokbd_state, &sc->viokbd_accents);

        if (action == NOKEY) {
                goto next_code;
        }

        return (action);
}

/* Check if char is waiting */
static int
viokbd_check_char(keyboard_t *kbd)
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
viokbd_ioctl(keyboard_t *kbd, u_long cmd, caddr_t arg)
{
        struct vtinput_softc *sc = kbd->kb_data;
        int error;

        switch (cmd) {
        case KDGKBMODE:
                *(int *)arg = sc->viokbd_mode;
                return (0);
        case KDSKBMODE:
                switch (*(int *)arg) {
                case K_XLATE:
                case K_RAW:
                case K_CODE:
                        sc->viokbd_mode = *(int *)arg;
                        return (0);
                default:
                        return (EINVAL);
                }
        /* -- MISSING IOCTLS NEEDED BY KBDMUX -- */
        case KDGETLED:
                *(int *)arg = 0; /* No physical LEDs */
                return (0);
        case KDSETLED:
                return (0);      /* Fake success for setting LEDs */
        case KDGKBSTATE:
                *(int *)arg = sc->viokbd_state;
                return (0);
        case KDSKBSTATE:
                sc->viokbd_state = *(int *)arg;
                return (0);
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
viokbd_lock(keyboard_t *kbd, int lock)
{
	return (1);
}

static void
viokbd_clear_state(keyboard_t *kbd)
{
        struct vtinput_softc *sc = kbd->kb_data;

        sc->viokbd_state = 0;
        sc->viokbd_accents = 0;
}

/* Save the internal state */
static int
viokbd_get_state(keyboard_t *kbd, void *buf, size_t len)
{
	return (len == 0) ? 1 : -1;
}

/* Set the internal state */
static int
viokbd_set_state(keyboard_t *kbd, void *buf, size_t len)
{
	return (EINVAL);
}

/* Set polling */
static int
viokbd_poll(keyboard_t *kbd, int on)
{
#if 0
	hv_kbd_sc *sc = kbd->kb_data;

	HVKBD_LOCK();
	/*
	 * Keep a reference count on polling to allow recursive
	 * cngrab() during a panic for example.
	 */
	if (on)
		sc->sc_polling++;
	else if (sc->sc_polling > 0)
		sc->sc_polling--;

	if (sc->sc_polling != 0) {
		sc->sc_flags |= HVKBD_FLAG_POLLING;
	} else {
		sc->sc_flags &= ~HVKBD_FLAG_POLLING;
	}
	HVKBD_UNLOCK();
#endif
	return (0);
}

/*
 * Local Variables:
 * c-file-style: "bsd"
 * End:
 */
