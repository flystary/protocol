#include "socket.h"
#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>

int initSocket()
{
    int lfd = socket(AF_INET, SOCK_STREAM, 0);
    if (lfd == -1)
    {
        perror("socket error");
        return -1;
    }
    return lfd;
}

void initSockaddr(struct sockaddr* addr, unsigned port, const char* ip)
{
    struct sockaddr_in* addrin = (struct sockaddr_in*)addr;
    addrin->sin_family = AF_INET;
    addrin->sin_port = htons(port);
    addrin->sin_addr.s_addr = inet_addr(ip);
}

int setListen(int lfd, unsigned port)
{
    struct sockaddr addr;
    initSockaddr(&addr, port, "0.0.0.0"); //INADDR_ANY

    int opt = 1;
    setsockopt(lfd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
    int ret = bind(lfd, &addr, sizeof(addr));
    if (ret == -1)
    {
        perror("bind error");
        return -1;
    }
    ret = listen(lfd, 128);
    if (ret == -1)
    {
        perror("listen error");
        return -1;
    }
    return 0;
}

int acceptConnect(int lfd, struct sockaddr* addr)
{
    int connfd;
    if (addr == NULL)
    {
        connfd = accept(lfd, NULL, NULL);
    } else {
        socklen_t len = sizeof(struct sockaddr);
        connfd = accept(lfd, addr, &len);
    }

    if (connfd == -1)
    {
        perror("accept error");
        return -1;
    }
    return connfd;
}

int connectToHost(int fd, unsigned port, const char* ip)
{
    struct sockaddr addr;
    initSockaddr(&addr, port, ip);
    int ret = connect(fd, &addr, sizeof(addr));
    if (ret == -1)
    {
        perror("connect error");
        return -1;
    }
    return 0;
}

int readn(int fd, char* buffer, int size)
{
    int left = size;
    int readBytes = 0;
    char* ptr = buffer;

    while (left)
    {
        readBytes = read(fd, ptr, left);
        if (readBytes == -1)
        {
            if (errno == EINTR)
            {
                readBytes = 0;
            } else {
                perror("read error");
                return -1;
            }
        } else if (readBytes == 0) {
            printf("对方主动断开了连接...\n");
            return -1;
        }
        left -= readBytes;
        ptr  += readBytes;
    }
    return size - left;
}

int writen(int fd, const char* buffer, int length)
{
    int left = length;
    int writeBytes = 0;
    const char* ptr = buffer;

    while(left)
    {
        writeBytes = write(fd, ptr, left);
        if (writeBytes <= 0){
            if (errno == EINTR)
            {
                writeBytes = 0;
            } else {
                perror("write error");
                return -1;
            }
        }
        ptr  += writeBytes;
        left -= writeBytes;  
    }
    return length;
}

int recvMessage(int fd, char** buffer, enum Type* t)
{
    int dataLen = 0;
    int ret = readn(fd, (char*)&dataLen, sizeof(int));
    if (ret == -1)
    {
        *buffer = NULL;
        return -1;
    }
    dataLen = ntohl(dataLen);
    char ch;
    readn(fd, &ch, 1);
    *t = ch == 'H' ? Heart : Message;
    char* tmpbuf = (char*)calloc(dataLen, sizeof(char));
    if (tmpbuf == NULL)
    {
        *buffer = NULL;
        return -1;
    }
    ret = readn(fd, tmpbuf, dataLen - 1);
    if (ret != dataLen - 1)
    {
        free(tmpbuf);
        *buffer = NULL;
        return -1;
    }
    *buffer = tmpbuf;
    return ret;
}

bool sendMessage(int fd, const char* buffer, int length, enum Type t)
{
    int dataLen = length + 1 + sizeof(int);
    char* data = (char*)malloc(dataLen);
    if (data == NULL)
    {
        return false;
    }

    int netlen = htonl(length + 1);
    memcpy(data, &netlen, sizeof(int));
    char* ch = t == Heart ? "H" : "M";
    memcpy(data + sizeof(int), ch, sizeof(char));
    memcpy(data + sizeof(int) + 1, buffer, length);
    int ret = writen(fd, data, dataLen);
    free(data);
    return ret == dataLen;
}
