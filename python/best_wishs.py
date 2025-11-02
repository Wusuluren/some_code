import tkinter as tk
import random
import sys
from tkinter import messagebox
import os

# 确保中文显示正常（PyCharm中通常无需额外配置，tkinter默认支持）
def show_warm_tip(root):
    window = tk.Toplevel(root)
    # 先隐藏窗口，完成布局后再显示，避免黑屏
    window.withdraw()
    # 获取屏幕尺寸并随机定位窗口
    screen_width = window.winfo_screenwidth()
    screen_height = window.winfo_screenheight()
    window_width = 250
    window_height = 80
    x = random.randint(0, screen_width - window_width)
    y = random.randint(0, screen_height - window_height)
    window.geometry(f"{window_width}x{window_height}+{x}+{y}")
    window.title("温馨提示")
    print(f"[popup] showing at {x},{y}", flush=True)
    
    # 提示语列表
    tips = [
        "多喝水哦~", "保持微笑", "每天都要元气满满",
        "记得吃水果", "保持好心情", "好好爱自己",
        "梦想成真", "金榜题名", "顺顺利利",
        "早点休息", "别熬夜", "天冷了，多穿衣服"
    ]
    tip = random.choice(tips)
    
    # 随机背景色
    bg_colors = [
        'lightpink', 'skyblue', 'lightgreen', 'lavender',
        'lightyellow', 'powderblue', 'mistyrose'
    ]
    bg = random.choice(bg_colors)
    
    # 添加标签
    label = tk.Label(
        window,
        text=tip,
        bg=bg,
        fg='black',  # 确保文字颜色为黑色，与浅色背景有对比
        font=('Heiti SC', 12),  # 使用macOS支持的黑体字体
        wraplength=230,  # 自动换行
        padx=10,
        pady=10
    )
    # 背景填充整个窗口，避免出现黑色背景
    window.configure(bg=bg)
    window.configure(highlightthickness=0, bd=0)
    label.configure(highlightthickness=0, bd=0)
    label.pack(fill='both', expand=True)
    
    # 窗口置顶
    window.attributes('-topmost', True)
    window.attributes('-alpha', 1.0)
    # window.overrideredirect(True)  # 移除窗口装饰，避免macOS上的渲染问题
    window.update_idletasks()
    window.deiconify()
    try:
        window.wait_visibility()
    except Exception:
        pass
    window.lift()
    try:
        window.focus_force()
    except Exception:
        pass
    try:
        window.bell()
    except Exception:
        pass
    
    # 空格键关闭当前窗口
    def close_window(event):
        window.destroy()
    window.bind('<space>', close_window)
    
    # 强制刷新窗口确保正确绘制
    window.update()
    # 3秒后自动关闭窗口（避免窗口堆积）
    window.after(30000, window.destroy)
    

if __name__ == '__main__':
    # 静默 macOS 上的 Tk 弃用警告
    os.environ["TK_SILENCE_DEPRECATION"] = "1"
    # 使用主线程中的 Tk root 调度多个弹窗
    root = tk.Tk()
    root.withdraw()
    print("[main] scheduling 5 popups...", flush=True)
    for i in range(100):
        delay = i * 1000
        print(f"[main] schedule popup {i+1} at +{delay}ms", flush=True)
        root.after(delay, lambda: show_warm_tip(root))
    # 所有弹窗计划完成后自动退出主循环
    total_duration_ms = (100 - 1) * 1000 + 3500
    root.after(total_duration_ms, lambda: (print("[main] quitting...", flush=True), root.quit()))
    root.mainloop()
