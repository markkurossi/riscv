/*-
 * SPDX-License-Identifier: BSD-3-Clause
 *
 * This header is BSD licensed so anyone can use the definitions to implement
 * compatible drivers/servers.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 * 3. Neither the name of IBM nor the names of its contributors
 *    may be used to endorse or promote products derived from this software
 *    without specific prior written permission.
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * ``AS IS'' AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED
 * TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
 * PURPOSE ARE DISCLAIMED.  IN NO EVENT SHALL IBM OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */

#ifndef _VIRTIO_INPUT_H
#define _VIRTIO_INPUT_H

#define VTINPUT_FIFOSZ 		128

struct vtinput_softc {
        struct fb_info		vtinput_fb_info;
        device_t		vtinput_dev;

        struct virtqueue       *vtinput_event_vq;
        struct virtqueue       *vtinput_status_vq;

        struct evdev_dev       *kbd;
        struct evdev_dev       *ptr;

        keyboard_t 	        vt_kbd;

        /* Keyboard FIFO and synchronization */
        struct mtx              vtinput_mtx;
        uint8_t                 vtinput_fifo[VTINPUT_FIFOSZ];
        u_int                   vtinput_fifo_head;
        u_int                   vtinput_fifo_tail;
        u_int                   vtinput_fifo_count;

        /* State machine tracking */
        int                     vtinput_prefix;
        int                     viokbd_state;
        int                     viokbd_accents;
        int                     viokbd_mode;
};

enum virtio_input_config_select {
        VIRTIO_INPUT_CFG_UNSET      = 0x00,
        VIRTIO_INPUT_CFG_ID_NAME    = 0x01,
        VIRTIO_INPUT_CFG_ID_SERIAL  = 0x02,
        VIRTIO_INPUT_CFG_ID_DEVIDS  = 0x03,
        VIRTIO_INPUT_CFG_PROP_BITS  = 0x10,
        VIRTIO_INPUT_CFG_EV_BITS    = 0x11,
        VIRTIO_INPUT_CFG_ABS_INFO   = 0x12,
};

struct virtio_input_absinfo {
        uint32_t  min;
        uint32_t  max;
        uint32_t  fuzz;
        uint32_t  flat;
        uint32_t  res;
};

struct virtio_input_devids {
        uint16_t  bustype;
        uint16_t  vendor;
        uint16_t  product;
        uint16_t  version;
};

struct virtio_input_config {
        uint8_t    select;
        uint8_t    subsel;
        uint8_t    size;
        uint8_t    reserved[5];
        union {
                char 	string[128];
                uint8_t	bitmap[128];
                struct virtio_input_absinfo abs;
                struct virtio_input_devids ids;
        } u;
};

struct virtio_input_event {
        uint16_t type;
        uint16_t code;
        uint32_t value;
};

/* Keyboard interface. */
int vtinput_kbd_driver_load(module_t, int, void *);
int vtinput_kbd_driver_attach(device_t);
int vtinput_kbd_driver_detach(device_t);
int vtinput_kbd_intr(keyboard_t *, void *);

#endif /* _VIRTIO_INPUT_H */

/*
 * Local Variables:
 * c-file-style: "bsd"
 * End:
 */
