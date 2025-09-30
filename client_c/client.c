#include <arpa/inet.h>
#include <netinet/in.h>
#include <stdio.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/un.h>
#include <unistd.h>

// POZOR: v tomto souboru je umyslne chyba, na cviceni bude opravena a pozdeji
// zverejnena
//        opravena verze

int main(void) {
  int client_socket;
  int return_value;
  char cbuf;
  int len_addr;
  struct sockaddr_in my_addr;

  // vytvoreni socketu
  // AF_INET - TCP/IP zasobnik
  // SOCK_STREAM - streamovane sluzby (spolehlive, spojovane)
  // 0 - vychozi protokol
  client_socket = socket(AF_INET, SOCK_STREAM, 0);

  if (client_socket <= 0) {
    printf("Socket ERR\n");
    return -1;
  }

  memset(&my_addr, 0, sizeof(struct sockaddr_in));

  // priprava struktury adresy - AF_INET adresa, port 10000, inet_addr - s
  // prekladem adresy z retezce do cisla zde je adresa protejsku, kam se chceme
  // pripojit
  my_addr.sin_family = AF_INET;
  my_addr.sin_port = htons(10000);
  my_addr.sin_addr.s_addr = inet_addr("127.0.0.1");

  // pripojeni se na vzdaleny server
  return_value = connect(client_socket, (struct sockaddr *)&my_addr,
                         sizeof(struct sockaddr_in));
  if (return_value == 0)
    printf("Connect - OK\n");
  else {
    printf("Connect - ERR\n");
    return -1;
  }

  // odesleme nejake bajty
  send(client_socket, "A", sizeof(char), 0);
  send(client_socket, "B", sizeof(char), 0);

  // uzavreme spojeni
  close(client_socket);

  return 0;
}
