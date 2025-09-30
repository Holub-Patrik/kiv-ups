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
  int server_socket = 0;
  int client_socket = 0;
  int return_value = 0;

  char cbuf[51];
  socklen_t len_addr;
  struct sockaddr_in my_addr, peer_addr;

  // vytvoreni socketu
  // AF_INET - TCP/IP zasobnik
  // SOCK_STREAM - streamovane sluzby (spolehlive, spojovane)
  // 0 - vychozi protokol
  server_socket = socket(AF_INET, SOCK_STREAM, 0);

  if (server_socket <= 0) {
    printf("Socket ERR\n");
    return -1;
  }

  memset(&my_addr, 0, sizeof(struct sockaddr_in));

  // priprava struktury adresy - AF_INET adresa, port 10000, INADDR_ANY =
  // jakakoliv adresa (vsechny) zde je adresa/port na ktere chceme poslouchat,
  // prijimat spojeni a komunikovat
  my_addr.sin_family = AF_INET;
  my_addr.sin_port = htons(10000);
  my_addr.sin_addr.s_addr = INADDR_ANY;

  // svazat socket s poslouchaci adresou
  return_value = bind(server_socket, (struct sockaddr *)&my_addr,
                      sizeof(struct sockaddr_in));

  if (return_value == 0)
    printf("Bind - OK\n");
  else {
    printf("Bind - ERR\n");
    return -1;
  }

  // vytvoreni fronty pro prichozi spojeni
  return_value = listen(server_socket, 5);

  if (return_value == 0)
    printf("Listen OK\n");
  else {
    printf("Listen ERR\n");
    return -1;
  }

  for (;;) {
    // prijeti spojeni
    // -- blokuje dokud neni nejake spojeni na vstupu
    client_socket =
        accept(server_socket, (struct sockaddr *)&peer_addr, &len_addr);

    // spojeni se podarilo prijmout
    if (client_socket > 0) {
      // paralelni procesy - fork
      // vytvori novy proces, rodicovskemu procesu vrati PID potomka, potomkovi
      // vrati nulu --> takto je muzeme rozlisit, potomka nechame zpracovat
      // pozadavek
      return_value = fork();
      if (return_value == 0) {
        // tato cast uz se vykona v potomkovi
        printf("Hura nove spojeni\n");
        do {
          // prijmout co jde
          return_value = recv(client_socket, &cbuf, 50, 0);
          printf("Prijato %c\n", cbuf);
        } while (return_value >
                 0); // dokud neco na vstupu je a dokud klient komunikuje

        // zavrit socket a ukoncit se (potomka) - rodic bezi dal!
        close(client_socket);
        return 0;
      } else // rodic - pouze za sebe zavre spojeni (potomek ho ma stale
             // otevrene dokud ho sam nezavre)
        close(client_socket);
    } else {
      // neco se nehezky sesypalo
      printf("Brutal Fatal ERROR\n");
      return -1;
    }
  }

  return 0;
}
