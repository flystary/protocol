#include <stdio.h>
#include "socket.h"
#include <string.h>
#include <pthread.h>
#include "clientlist.h"
#include <stdlib.h>
#include <unistd.h>

pthread_mutex_t mutex;
struct FdInfo
{
    int fd;
    int count;  // 记录有多少次没有收到服务器回复的心跳包数据
};

void* parseRecvMessage(void* arg)
{
    struct ClientInfo* info = (struct ClientInfo*)arg;
    while (1)
    {
        char* buffer;
        enum Type t;
        int len = recvMessage(info->fd, &buffer, &t);
        if (buffer == NULL)
        {
            printf("fd = %d, 通信的子线程退出了...\n", info->fd);
            pthread_exit(NULL);
        }
        else
        {
            if (t == Heart)
            {
                printf("心跳包...%s\n", buffer);
                pthread_mutex_lock(&mutex);
                info->count = 0;
                pthread_mutex_unlock(&mutex);
                sendMessage(info->fd, buffer, len, Heart);
            }
            else
            {
                const char* pt = "Hello...............";
                printf("数据包: %s\n", buffer);
                sendMessage(info->fd, pt, strlen(pt), Message);
            }
            free(buffer);
        }
    }
    return NULL;
}

// 1. 发送心跳包数据
// 2. 检测心跳包, 看看能否收到服务器回复的数据
void* heartBeat(void* arg)
{
    struct ClientInfo* head = (struct ClientInfo*)arg;
    struct ClientInfo* p = NULL;
    while (1)
    {
        p = head->next;
        while (p)
        {
            pthread_mutex_lock(&mutex);
            p->count++;    // 默认没收到服务器回复的心跳包数据
            printf("fd = %d, count = %d\n", p->fd, p->count);
            if (p->count > 5)
            {
                // 客户端和服务器断开了连接
                printf("客户端 fd = %d 和服务器断开了连接...\n", p->fd);
                close(p->fd);
                // 释放套接字资源, 
                pthread_cancel(p->pid);
                removeNode(head, p->fd);
            }
            pthread_mutex_unlock(&mutex);
            p = p->next;
        }
        sleep(3);
    }
    return NULL;
}

int main()
{
    unsigned short port = 8888;
    int lfd = initSocket();
    setListen(lfd, port);
    // 创建链表
    struct ClientInfo* head = createList();
    pthread_mutex_init(&mutex, NULL);


    // 添加心跳包子线程
    pthread_t pid1;
    pthread_create(&pid1, NULL, heartBeat, head);


    while (1)
    {
        int sockfd = acceptConnect(lfd, NULL);
        if (sockfd == -1)
        {
            continue;
        }
        struct ClientInfo* node = prependNode(head, sockfd);
        // 创建接收数据的子线程
        pthread_create(&node->pid, NULL, parseRecvMessage, node);
        pthread_detach(node->pid);
    }

    pthread_join(pid1, NULL);
    pthread_mutex_destroy(&mutex);
    close(lfd);
    return 0;
}
