#include <stdio.h>
#include "socket.h"
#include <string.h>
#include <pthread.h>
#include <stdlib.h>
#include <unistd.h>

pthread_mutex_t mutex;

struct FdInfo
{
    int     fd;
    int     count;
};

void* parseRecvMessage(void* arg)
{
    struct FdInfo* info = (struct FdInfo*)arg;
    while(1)
    {
        char*   buffer;
        enum  Type t;
        recvMessage(info->fd, &buffer, &t);
        if (buffer == NULL)
        {
            continue;
        } else {
            if (t == Heart)
            {
                printf("心跳包...%s\n", buffer);
                pthread_mutex_lock(&mutex);
                info->count = 0;
                pthread_mutex_unlock(&mutex);
            } else {
                printf("数据包: %s\n", buffer);    
            }
            free(buffer);
        }
    }
    return NULL;
}

void* heartBeat(void* arg)
{
    struct FdInfo* info = (struct FdInfo*)arg;
    while(1)
    {
        pthread_mutex_lock(&mutex);
        info->count++;
        printf("fd = %d, cout = %d\n", info->fd, info->count);
        if (info->count > 5)
        {
            printf("客户端与服务器断开了连接....\n");
            close(info->fd);
            exit(0);
        }
        pthread_mutex_unlock(&mutex);
        sendMessage(info->fd, "hello", 5, Heart);
        sleep(3);
    }
    return NULL;
}

int main()
{
    struct FdInfo info;
    unsigned short port = 8888;
    const char* ip = "127.0.0.1";
    info.fd = initSocket();
    info.count = 0;
    connectToHost(info.fd, port, ip);

    pthread_mutex_init(&mutex, NULL);
    pthread_t pid;
    pthread_create(&pid, NULL, parseRecvMessage, &info);

    pthread_t pid1;
    pthread_create(&pid1, NULL, heartBeat, &info);

    while(1)
    {
        const char* data = "你好! .......";
        sendMessage(info.fd, data, strlen(data), Message);
        sleep(2);
    }

    pthread_join(pid, NULL);
    pthread_join(pid1, NULL);
    pthread_mutex_destroy(&mutex);
    return 0;
}
