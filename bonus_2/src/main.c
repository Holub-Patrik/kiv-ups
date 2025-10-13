#include <math.h>
#include <netinet/in.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/un.h>
#include <time.h>
#include <unistd.h>

#define MSG_BUF 256
#define HELLO_LEN 6
#define RANDOM_RANGE 10000
#define NUM_PREFIX_LEN 4
#define OK_LEN 3
#define WRONG_LEN 6

// telo vlakna co obsluhuje prichozi spojeni
void *serve_request(void *arg) {
  int client_sock;
  char msg_buf[MSG_BUF];
  const char num_prefix[NUM_PREFIX_LEN] = {'N', 'U', 'M', ':'};
  const char ok_str[OK_LEN] = {'O', 'K', '\n'};
  const char wrong_str[WRONG_LEN] = {'W', 'R', 'O', 'N', 'G', '\n'};
  char *msg_to_client;
  int msg_to_client_len;
  int i;
  ssize_t bytes_read;
  const char *expected_hello = "HELLO\n";
  long random_value;
  int random_value_length;
  long double_value;

  memset(msg_buf, 0, MSG_BUF);

  // pretypujem parametr z netypoveho ukazate na ukazatel na int a dereferujeme
  // --> to nam vrati puvodni socket
  client_sock = *(int *)arg;

  i = 0;
  while (i < HELLO_LEN) {
    bytes_read = recv(client_sock, &msg_buf[i], sizeof(char), 0);
    if (bytes_read == -1) {
      puts("FATAL ERROR");
      exit(1);
    }

    if (bytes_read < 1) {
      continue;
    }

    i++;
  }

  if (memcmp(msg_buf, expected_hello, HELLO_LEN)) {
    puts("Client didn't send correct HELLO");
    free(arg);
    return (void *)1;
  }

  puts("sending random value");

  random_value = random() % RANDOM_RANGE;
  if (random_value == 0) {
    random_value_length = 1;
  } else {
    random_value_length = floor(log10(labs(random_value))) + 1;

    if (random_value < 0) {
      // techincally shouldn't happen, but if I ever change the range and
      // wouldn't put this in, the length would be incorrect
      random_value_length += 1;
    }
  }

  msg_to_client_len = NUM_PREFIX_LEN + random_value_length + 1;
  msg_to_client = malloc(msg_to_client_len + 1);

  snprintf(msg_to_client, msg_to_client_len + 1, "NUM:%ld\n", random_value);
  send(client_sock, msg_to_client, msg_to_client_len * sizeof(char), 0);

  puts("sent random value");

  i = 0;
  while (i < MSG_BUF) {
    bytes_read = recv(client_sock, &msg_buf[i], sizeof(char), 0);
    if (bytes_read == -1) {
      puts("FATAL ERROR");
      exit(1);
    }

    if (bytes_read < 1) {
      continue;
    }

    if (msg_buf[i] == '\n') {
      break;
    }

    i++;
  }

  if (memcmp(msg_buf, num_prefix, NUM_PREFIX_LEN)) {
    send(client_sock, &wrong_str, sizeof(char) * WRONG_LEN, 0);

    free(msg_to_client);
    free(arg);
    return (void *)0;
  }

  double_value = strtol(&msg_buf[NUM_PREFIX_LEN], NULL, 10);

  if (double_value == random_value * 2) {
    send(client_sock, &ok_str, sizeof(char) * OK_LEN, 0);
  } else {
    send(client_sock, &wrong_str, sizeof(char) * WRONG_LEN, 0);
  }

  printf("Exitting ...\n");

  close(client_sock);

  free(msg_to_client);
  free(arg);
  return (void *)0;
}

int main(void) {
  int server_sock;
  int client_sock;
  int return_value;
  int param;
  int *th_socket;
  struct sockaddr_in local_addr;
  struct sockaddr_in remote_addr;
  socklen_t remote_addr_len;
  pthread_t thread_id;

  srand(time(0)); // initialize random number generator

  server_sock = socket(AF_INET, SOCK_STREAM, 0);

  if (server_sock <= 0) {
    printf("Socket ERR\n");
    return -1;
  }

  memset(&local_addr, 0, sizeof(struct sockaddr_in));

  local_addr.sin_family = AF_INET;
  local_addr.sin_port = htons(10000);
  local_addr.sin_addr.s_addr = INADDR_ANY;
  // local_addr.sin_addr.s_addr = inet_addr("147.228.67.10");

  // nastavime parametr SO_REUSEADDR - "znovupouzije" puvodni socket, co jeste
  // muze hnit v systemu bez predchoziho close
  param = 1;
  return_value = setsockopt(server_sock, SOL_SOCKET, SO_REUSEADDR,
                            (const char *)&param, sizeof(int));

  if (return_value == -1)
    printf("setsockopt ERR\n");

  return_value = bind(server_sock, (struct sockaddr *)&local_addr,
                      sizeof(struct sockaddr_in));

  if (return_value == 0)
    printf("Bind OK\n");
  else {
    printf("Bind ERR\n");
    return -1;
  }

  return_value = listen(server_sock, 5);
  if (return_value == 0)
    printf("Listen OK\n");
  else {
    printf("Listen ERR\n");
    return -1;
  }

  while (1) {
    client_sock =
        accept(server_sock, (struct sockaddr *)&remote_addr, &remote_addr_len);

    if (client_sock > 0) {
      // misto forku vytvorime nove vlakno - je potreba alokovat pamet, predat
      // ridici data (zde jen socket) a vlakno spustit

      th_socket = malloc(sizeof(int));
      *th_socket = client_sock;
      pthread_create(&thread_id, NULL, (void *(*)(void *)) & serve_request,
                     (void *)th_socket);
    } else {
      printf("Brutal Fatal ERROR\n");
      return -1;
    }
  }

  return EXIT_SUCCESS;
}
