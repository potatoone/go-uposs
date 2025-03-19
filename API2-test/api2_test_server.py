from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import time
from datetime import datetime

# 配置
PORT = 3001
LOG_FILE = "api2_requests.log"

class API2Handler(BaseHTTPRequestHandler):
    def _set_headers(self, status_code=200):
        self.send_response(status_code)
        self.send_header('Content-type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        self.end_headers()
    
    def do_OPTIONS(self):
        self._set_headers()
        
    def do_POST(self):
        timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        print(f"[{timestamp}] 收到POST请求: {self.path}")
        
        # 读取请求内容
        content_length = int(self.headers.get('Content-Length', 0))
        post_data = self.rfile.read(content_length)
        
        try:
            # 尝试解析JSON
            json_data = json.loads(post_data.decode('utf-8'))
            print(f"请求体: {json.dumps(json_data, indent=2, ensure_ascii=False)}")
            
            # 检查是否包含必要的字段
            order_number = json_data.get("orderNumber", "")  # 改为单数形式
            file_url = json_data.get("fileURL", "")
            
            # 记录到控制台
            print(f"订单号: {order_number}")  # 修改为单数
            print(f"文件URL: {file_url}")
            
            # 写入日志文件
            log_entry = {
                "timestamp": timestamp,
                "orderNumber": order_number,
                "fileURL": file_url
            }
            with open(LOG_FILE, 'a', encoding='utf-8') as f:
                f.write(json.dumps(log_entry, ensure_ascii=False) + "\n")
            
            # 返回成功响应
            self._set_headers()
            response = {
                "code": 200,
                "message": "推送成功",
                "receivedData": {
                    "orderNumber": order_number,
                    "fileURL": file_url
                }
            }
            self.wfile.write(json.dumps(response, ensure_ascii=False).encode())
            
        except json.JSONDecodeError:
            print("错误: 无效的JSON格式")
            self._set_headers(400)
            response = {
                "code": 400, 
                "message": "无效的JSON格式"
            }
            self.wfile.write(json.dumps(response, ensure_ascii=False).encode())
        except Exception as e:
            print(f"处理请求时出错: {str(e)}")
            self._set_headers(500)
            response = {
                "code": 500,
                "message": f"服务器内部错误: {str(e)}"
            }
            self.wfile.write(json.dumps(response, ensure_ascii=False).encode())

def run_server():
    server_address = ('', PORT)
    httpd = HTTPServer(server_address, API2Handler)
    print(f"API2 测试服务器启动在 http://localhost:{PORT}")
    print("推送接口: POST http://localhost:3001")
    # 修改预期数据结构
    print('预期数据格式: { "orderNumber": "订单号", "fileURL": "图片URL" }')
    httpd.serve_forever()

if __name__ == '__main__':
    run_server()