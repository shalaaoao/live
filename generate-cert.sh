#!/bin/bash

# 生成自签名证书脚本（支持 IP 地址访问）

echo "正在生成自签名证书..."

# 获取本机 IP 地址
LOCAL_IP=$(ifconfig | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1' | head -1)

if [ -z "$LOCAL_IP" ]; then
    LOCAL_IP="127.0.0.1"
fi

echo "检测到本机 IP: $LOCAL_IP"

# 创建证书配置文件
cat > cert.conf <<EOF
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = v3_req

[dn]
C=CN
ST=State
L=City
O=Organization
CN=localhost

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.local
IP.1 = 127.0.0.1
IP.2 = $LOCAL_IP
EOF

# 生成私钥
openssl genrsa -out server.key 2048

# 生成证书签名请求
openssl req -new -key server.key -out server.csr -config cert.conf

# 生成自签名证书（有效期365天）
openssl x509 -req -days 365 -in server.csr -signkey server.key -out server.crt -extensions v3_req -extfile cert.conf

# 清理临时文件
rm server.csr cert.conf

echo ""
echo "证书生成完成！"
echo "文件："
echo "  - server.key (私钥)"
echo "  - server.crt (证书)"
echo ""
echo "支持的访问地址："
echo "  - https://localhost:8080"
echo "  - https://127.0.0.1:8080"
echo "  - https://$LOCAL_IP:8080"
echo ""
echo "注意：这是自签名证书，浏览器会显示安全警告，需要手动点击'继续'或'高级'->'继续访问'"
