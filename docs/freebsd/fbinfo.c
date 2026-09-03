/*
 * Copyright (c) 2026 Markku Rossi
 *
 * All rights reserved.
 */

#include <sys/types.h>
#include <sys/ioctl.h>
#include <sys/fbio.h>
#include <sys/mman.h>

#include <err.h>
#include <fcntl.h>
#include <stdio.h>
#include <unistd.h>
#include <stdint.h>
#include <math.h>

#include <goemu.h>

typedef struct
{
  uint8_t r;
  uint8_t g;
  uint8_t b;
} RGBColor;

RGBColor
hsv_to_rgb(double h, double s, double v)
{
  double c = v * s;
  double x = c * (1.0 - fabs(fmod(h / 60.0, 2.0) - 1.0));
  double m = v - c;
  double r_prime = 0, g_prime = 0, b_prime = 0;
  RGBColor color;

  if (h >= 0 && h < 60)
    {
      r_prime = c;
      g_prime = x;
      b_prime = 0;
    }
  else if (h >= 60 && h < 120)
    {
      r_prime = x;
      g_prime = c;
      b_prime = 0;
    }
  else if (h >= 120 && h < 180)
    {
      r_prime = 0;
      g_prime = c;
      b_prime = x;
    }
  else if (h >= 180 && h < 240)
    {
      r_prime = 0;
      g_prime = x;
      b_prime = c;
    }
  else if (h >= 240 && h < 300)
    {
      r_prime = x;
      g_prime = 0;
      b_prime = c;
    }
  else if (h >= 300 && h < 360)
    {
      r_prime = c;
      g_prime = 0;
      b_prime = x;
    }

  color.r = (uint8_t)((r_prime + m) * 255.0);
  color.g = (uint8_t)((g_prime + m) * 255.0);
  color.b = (uint8_t)((b_prime + m) * 255.0);

  return color;
}

void
print_stats(size_t size, unsigned long ns)
{
  double s = (double) ns / 1000000000.0;

  printf("%ld pixels in %.2fs, %.2f pixels/s\n", size, s, (double) size / s);
}

int
main(int argc, char **argv)
{
  const char *device = "/dev/ttyv8";
  struct fbtype fb;
  u_int stride;
  size_t size;
  unsigned char *buf;
  size_t i, x, y;
  unsigned long ns;

  if (argc > 2)
    errx(1, "usage: %s [tty-device]", argv[0]);

  if (argc == 2)
    device = argv[1];

  int fd = open(device, O_RDWR);
  if (fd < 0)
    err(1, "open %s", device);

  if (ioctl(fd, FBIOGTYPE, &fb) < 0)
    err(1, "FBIOGTYPE");

  if (ioctl(fd, FBIO_GETLINEWIDTH, &stride) < 0)
    err(1, "FBIO_GETLINEWIDTH");

  size = (size_t) stride * (size_t) fb.fb_height;

  printf("device:  %s\n", device);
  printf("width:   %d pixels\n", fb.fb_width);
  printf("height:  %d pixels\n", fb.fb_height);
  printf("depth:   %d bits\n", fb.fb_depth);
  printf("stride:  %u bytes\n", stride);
  printf("size:    %zu bytes (0x%zx)\n", size, size);

  buf = mmap(NULL, size, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
  if (buf == NULL)
    errx(1, "mmap");

  ns = time_ns();
  for (i = 0; i < size; i++)
    buf[i] = 0x00;
  ns = time_ns() - ns;

  print_stats(size / 4, ns);

  ns = time_ns();
  for (y = 0; y < fb.fb_height; y++)
    for (x = 0; x < fb.fb_width; x++)
      {
        double h = ((double) x / (fb.fb_width - 1)) * 360.0; // X: Hue [0, 360]
        double s = 1.0;         // Saturation
        // Y: Value [1 down to 0]
        double v = 1.0 - ((double) y / (fb.fb_height - 1));
        RGBColor color = hsv_to_rgb(h, s, v);

        buf[y * stride + x * 4 + 0] = color.b;
        buf[y * stride + x * 4 + 1] = color.g;
        buf[y * stride + x * 4 + 2] = color.r;
        buf[y * stride + x * 4 + 3] = 0xff;
      }
  ns = time_ns() - ns;
  print_stats(size / 4, ns);

  close(fd);

  return 0;
}
