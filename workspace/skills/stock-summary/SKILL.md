---
name: stock-summary
description: Tóm tắt nhanh 1 mã cổ phiếu (giá + ngữ cảnh ngắn). Dùng khi user hỏi giá hoặc overview 1 ticker.
---

# Stock summary

Khi user hỏi về một mã (vd AAPL, VNM.VN):

1. Gọi tool get_stock_quote với symbol đó.
2. Tóm tắt ngắn: giá gần nhất, đơn vị tiền, và 1-2 câu lưu ý (không phải lời khuyên đầu tư).
3. Nếu thiếu symbol, hỏi lại user.
