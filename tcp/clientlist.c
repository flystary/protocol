#include "clientlist.h"
#include <stdio.h>
#include <stdlib.h>

struct ClientInfo* createList()
{
    struct ClientInfo* head = (struct ClientInfo*)malloc(sizeof(struct ClientInfo));
    return head;
}

struct ClientInfo* prependNode(struct ClientInfo* head, int fd)
{
    struct ClientInfo* node = (struct ClientInfo*)malloc(sizeof(struct ClientInfo));
    node->fd = fd;
    node->next = head->next;
    head->next = node;
    return node;
}

bool removeNode(struct ClientInfo* head, int fd)
{
    struct ClientInfo* p = head;
    struct ClientInfo* q = head->next;
    while (q)
    {
        if (q->fd == fd)
        {
            p->next = q->next;
            free(q);
            printf("成功过将链表中的 fd 节点删除了, fd = %d\n", fd);
            return true;
        }
        p = p->next;
        q = q->next;
    }
    return false;
}

void freeClientList(struct ClientInfo* head)
{
    struct ClientInfo* p = head;
    while (p)
    {
        struct ClientInfo* tmp = p;
        p = p->next;
        free(tmp);
    }
}

