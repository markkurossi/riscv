/*
  * pthread.c
  */

#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include <unistd.h>

// Shared resources
int buffer;
int is_full = 0; // 0 = empty, 1 = full

// Synchronization primitives
pthread_mutex_t mutex = PTHREAD_MUTEX_INITIALIZER;
pthread_cond_t cond_empty = PTHREAD_COND_INITIALIZER;
pthread_cond_t cond_full = PTHREAD_COND_INITIALIZER;

void *
producer(void* arg)
{
  int i;

  for (i = 0; i < 10; i++)
    {
      pthread_mutex_lock(&mutex);

      // Wait while the buffer is full
      while (is_full)
        pthread_cond_wait(&cond_empty, &mutex);

      // Produce the item
      buffer = i;
      is_full = 1;
      printf("Producer: Produced %d\n", buffer);

      // Signal the consumer and release mutex
      pthread_cond_signal(&cond_full);
      pthread_mutex_unlock(&mutex);

      sleep(1); // Small delay for visibility
    }

  return NULL;
}

void *
consumer(void* arg)
{
  int i;

  for (i = 0; i < 10; i++)
    {
      pthread_mutex_lock(&mutex);

      // Wait while the buffer is empty
      while (!is_full)
        pthread_cond_wait(&cond_full, &mutex);

      // Consume the item
      int data = buffer;
      is_full = 0;
      printf("Consumer: Consumed %d\n", data);

      // Signal the producer and release mutex
      pthread_cond_signal(&cond_empty);
      pthread_mutex_unlock(&mutex);

      usleep(500000); // Consumer works slightly faster
    }

  return NULL;
}

int main()
{
  pthread_t prod_tid, cons_tid;

  // Create threads
  pthread_create(&prod_tid, NULL, producer, NULL);
  pthread_create(&cons_tid, NULL, consumer, NULL);

  // Wait for threads to finish
  pthread_join(prod_tid, NULL);
  pthread_join(cons_tid, NULL);

  // Cleanup
  pthread_mutex_destroy(&mutex);
  pthread_cond_destroy(&cond_empty);
  pthread_cond_destroy(&cond_full);

  printf("Finished processing.\n");
  return 0;
}
