import { Solar, Lunar, HolidayUtil } from 'lunar-javascript';

export async function renderWatermark(data: any): Promise<string> {
  console.log("renderWatermark called with:", { ...data, image_path: data.image_path ? "data:..." : undefined });
  const { 
    image_path, title, date, copyright, variant, 
    enable_watermark, enable_calendar, holiday_data,
    only_overlay, width, height
  } = data;

  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Could not get canvas context');

  let targetWidth = width;
  let targetHeight = height;

  if (image_path) {
    const img = new Image();
    img.crossOrigin = "anonymous";
    img.src = image_path;
    
    await new Promise((resolve, reject) => {
      img.onload = resolve;
      img.onerror = reject;
    });
    
    if (!targetWidth) targetWidth = img.width;
    if (!targetHeight) targetHeight = img.height;
    
    canvas.width = targetWidth;
    canvas.height = targetHeight;

    if (!only_overlay) {
      // Draw original image
      ctx.drawImage(img, 0, 0, targetWidth, targetHeight);
    }
  } else {
    if (!targetWidth || !targetHeight) {
       throw new Error("Width and height required when no image_path provided");
    }
    canvas.width = targetWidth;
    canvas.height = targetHeight;
  }

  // 1. Draw Watermark if enabled
  if (enable_watermark) {
    drawWatermark(ctx, canvas, title, date, copyright, variant);
  }

  // 2. Draw Calendar if enabled
  if (enable_calendar) {
    console.log("Drawing calendar...");
    drawCalendar(ctx, canvas, date, holiday_data);
  }

  return canvas.toDataURL(only_overlay ? 'image/png' : 'image/jpeg', only_overlay ? undefined : 0.95);
}

function drawWatermark(ctx: CanvasRenderingContext2D, canvas: HTMLCanvasElement, title: string, date: string, copyright: string, variant: string) {
  // Watermark style
  const paddingX = canvas.width * 0.05;
  const paddingY = canvas.height * 0.05;
  const titleFontSize = Math.max(24, Math.floor(canvas.height * 0.045));
  const copyrightFontSize = Math.max(14, Math.floor(canvas.height * 0.018));
  const tagFontSize = Math.max(12, Math.floor(canvas.height * 0.015));

  // Draw bottom gradient
  const gradient = ctx.createLinearGradient(0, canvas.height * 0.7, 0, canvas.height);
  gradient.addColorStop(0, 'rgba(0, 0, 0, 0)');
  gradient.addColorStop(1, 'rgba(0, 0, 0, 0.6)');
  ctx.fillStyle = gradient;
  ctx.fillRect(0, canvas.height * 0.7, canvas.width, canvas.height * 0.3);

  // Reset shadow for text
  ctx.shadowColor = 'rgba(0, 0, 0, 0.8)';
  ctx.shadowBlur = 12;
  ctx.shadowOffsetX = 2;
  ctx.shadowOffsetY = 2;

  // Draw Title
  ctx.font = `bold ${titleFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.fillStyle = 'white';
  ctx.textBaseline = 'bottom';
  const titleY = canvas.height - paddingY - (copyrightFontSize * 2.5);
  ctx.fillText(title, paddingX, titleY);

  // Draw Copyright
  ctx.shadowBlur = 8;
  ctx.font = `${copyrightFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
  const copyrightY = canvas.height - paddingY - (tagFontSize * 2.0);
  ctx.fillText(copyright, paddingX, copyrightY);

  // Draw Tags
  ctx.shadowBlur = 4;
  ctx.font = `${tagFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  
  const tags = [date, variant || "UHD"];
  let currentTagX = paddingX;
  const tagY = canvas.height - paddingY;
  const tagPaddingH = tagFontSize * 0.8;
  const tagPaddingV = tagFontSize * 0.3;
  const tagRadius = 4;

  tags.forEach(tag => {
    const tagWidth = ctx.measureText(tag).width;
    const rectWidth = tagWidth + tagPaddingH * 2;
    const rectHeight = tagFontSize + tagPaddingV * 2;
    const rectX = currentTagX;
    const rectY = tagY - rectHeight;

    ctx.save();
    ctx.shadowBlur = 0;
    ctx.shadowOffsetX = 0;
    ctx.shadowOffsetY = 0;
    ctx.fillStyle = 'rgba(0, 0, 0, 0.4)';
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.1)';
    ctx.lineWidth = 1;
    
    // Rounded rect
    ctx.beginPath();
    // @ts-ignore
    if (ctx.roundRect) {
      // @ts-ignore
      ctx.roundRect(rectX, rectY, rectWidth, rectHeight, tagRadius);
    } else {
      ctx.rect(rectX, rectY, rectWidth, rectHeight);
    }
    ctx.fill();
    ctx.stroke();
    ctx.restore();

    ctx.fillStyle = 'rgba(255, 255, 255, 0.6)';
    ctx.fillText(tag, rectX + tagPaddingH, tagY - tagPaddingV);
    currentTagX += rectWidth + 12;
  });
}

function drawCalendar(ctx: CanvasRenderingContext2D, canvas: HTMLCanvasElement, dateStr: string, holidayData: any[]) {
  try {
    // 解析日期
    const dateParts = dateStr.split('-');
    let year: number, month: number, today: number;
    if (dateParts.length === 3) {
      year = parseInt(dateParts[0]);
      month = parseInt(dateParts[1]);
      today = parseInt(dateParts[2]);
    } else {
      const dateObj = new Date(dateStr);
      year = dateObj.getFullYear();
      month = dateObj.getMonth() + 1;
      today = dateObj.getDate();
    }

    if (isNaN(year) || isNaN(month) || isNaN(today)) {
      throw new Error(`Invalid date: ${dateStr}`);
    }

    console.log(`Calendar for: ${year}-${month}-${today}`);

    // 日历配置
    const scale = canvas.height / 1080;
    const padding = 30 * scale;
    const boxW = 320 * scale;
    const boxX = canvas.width - boxW - padding;
    const boxY = padding;
    
    // 计算网格和高度
    const firstDay = new Date(year, month - 1, 1).getDay(); // 0-6
    const lastDay = new Date(year, month, 0).getDate();
    const rowCount = Math.ceil((firstDay + lastDay) / 7);
    const cellH = 40 * scale;
    const boxH = (110 + rowCount * 40 + 10) * scale; // 动态计算高度

    // 1. 背景磨砂质感
    ctx.save();
    ctx.fillStyle = 'rgba(0, 0, 0, 0.4)';
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.15)';
    ctx.lineWidth = Math.max(1, 1 * scale);
    const radius = 12 * scale;
    
    ctx.beginPath();
    // @ts-ignore
    if (ctx.roundRect) {
      // @ts-ignore
      ctx.roundRect(boxX, boxY, boxW, boxH, radius);
    } else {
      ctx.rect(boxX, boxY, boxW, boxH);
    }
    ctx.fill();
    ctx.stroke();
    ctx.restore();

    // 2. 渲染标题（公历年月日及农历）
    ctx.save();
    ctx.fillStyle = 'white';
    ctx.textBaseline = 'top';
    ctx.textAlign = 'left';
    
    // 公历日期 (YYYY年MM月DD日)
    const dateFontSize = 20 * scale;
    ctx.font = `bold ${dateFontSize}px "Segoe UI", Roboto, sans-serif`;
    const dateText = `${year}年${month}月${today}日`;
    ctx.fillText(dateText, boxX + 18 * scale, boxY + 15 * scale);
    
    // 农历日期
    const solar = Solar.fromYmd(year, month, today);
    const lunar = Lunar.fromSolar(solar);
    const lunarFontSize = 11 * scale;
    ctx.font = `${lunarFontSize}px sans-serif`;
    ctx.fillStyle = 'rgba(255, 255, 255, 0.7)';
    const lunarText = `农历 ${lunar.getYearInGanZhi()}${lunar.getYearShengXiao()}年 · ${lunar.getMonthInChinese()}月${lunar.getDayInChinese()}`;
    ctx.fillText(lunarText, boxX + 18 * scale, boxY + 15 * scale + dateFontSize + 6 * scale);
    
    // 3. 星期标题
    const weekDays = ["日", "一", "二", "三", "四", "五", "六"];
    const cellW = (boxW - 24 * scale) / 7;
    const startY = boxY + 85 * scale;
    
    ctx.font = `bold ${12 * scale}px sans-serif`;
    ctx.textAlign = 'center';
    weekDays.forEach((day, i) => {
      ctx.fillStyle = (i === 0 || i === 6) ? 'rgba(255, 120, 120, 1)' : 'rgba(255, 255, 255, 0.5)';
      ctx.fillText(day, boxX + 12 * scale + i * cellW + cellW / 2, startY);
    });
    
    // 4. 计算本月日期网格
    const dateStartY = startY + 25 * scale;
    
    for (let i = 1; i <= lastDay; i++) {
      const dayIdx = i + firstDay - 1;
      const row = Math.floor(dayIdx / 7);
      const col = dayIdx % 7;
      
      const x = boxX + 12 * scale + col * cellW + cellW / 2;
      const y = dateStartY + row * cellH;
      
      // 是否是今天
      const isToday = i === today;
      if (isToday) {
        ctx.save();
        ctx.fillStyle = 'rgba(255, 255, 255, 0.25)';
        ctx.beginPath();
        // @ts-ignore
        if (ctx.roundRect) {
          // @ts-ignore
          ctx.roundRect(x - cellW/2.2, y - cellH/2.2, cellW/1.1, cellH/1.1, 6 * scale);
        } else {
          ctx.arc(x, y, 16 * scale, 0, Math.PI * 2);
        }
        ctx.fill();
        ctx.restore();
      }
      
      // 获取农历/节假日信息
      try {
        const solar = Solar.fromYmd(year, month, i);
        const lunar = Lunar.fromSolar(solar);
        
        // 判断是否是节假日或调休
        let holidayMark = ""; // "休" or "班"
        let holidayName = "";
        
        // 先查后端传入的 holidayData (优先级高)
        const currentFullDate = `${year}-${month.toString().padStart(2, '0')}-${i.toString().padStart(2, '0')}`;
        const holidayInfo = holidayData?.find((d: any) => d.date === currentFullDate);
        
        if (holidayInfo) {
          holidayMark = holidayInfo.isOffDay ? "休" : "班";
          if (holidayInfo.isOffDay) holidayName = holidayInfo.name;
        } else {
          // 默认逻辑：周六周日为休
          if (col === 0 || col === 6) {
            holidayMark = "休";
          }
        }
        
        // 渲染公历
        ctx.font = `bold ${16 * scale}px sans-serif`;
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = isToday ? 'white' : 'rgba(255, 255, 255, 0.9)';
        if (holidayMark === "休" && !isToday) {
            ctx.fillStyle = 'rgba(255, 150, 150, 1)';
        }
        ctx.fillText(i.toString(), x, y - 2 * scale);
        
        // 渲染农历或节气或节日
        ctx.font = `${9 * scale}px sans-serif`;
        let subText = lunar.getDayInChinese();
        
        // 优先级：法定节日 > 节气 > 农历日期
        const jieQi = lunar.getJieQi();
        if (jieQi) subText = jieQi;
        
        const festivals = lunar.getFestivals();
        if (festivals.length > 0) {
          const f = festivals[0];
          if (f.length <= 4) subText = f;
        }
        
        if (holidayName) subText = holidayName;
        
        // 截断太长的文字
        if (subText.length > 3) subText = subText.substring(0, 3);
        
        ctx.fillStyle = isToday ? 'rgba(255, 255, 255, 0.85)' : 'rgba(255, 255, 255, 0.45)';
        if (holidayMark === "休" && !isToday) {
            ctx.fillStyle = 'rgba(255, 150, 150, 0.7)';
        }
        ctx.fillText(subText, x, y + 13 * scale);
        
        // 渲染“休/班”标记
        if (holidayMark) {
          ctx.save();
          ctx.font = `bold ${8 * scale}px sans-serif`;
          const markX = x + cellW/2.8;
          const markY = y - cellH/2.8;
          ctx.fillStyle = holidayMark === "休" ? 'rgba(255, 80, 80, 0.9)' : 'rgba(100, 255, 100, 0.9)';
          ctx.fillText(holidayMark, markX, markY + 8 * scale);
          ctx.restore();
        }
      } catch (err) {
        // 如果 lunar 报错，至少渲染公历
        ctx.font = `bold ${16 * scale}px sans-serif`;
        ctx.fillStyle = 'white';
        ctx.fillText(i.toString(), x, y);
      }
    }
    ctx.restore();
  } catch (err) {
    console.error("Error drawing calendar:", err);
    // 渲染简单的错误提示
    ctx.fillStyle = 'rgba(255, 0, 0, 0.5)';
    ctx.fillRect(canvas.width - 200, 0, 200, 50);
    ctx.fillStyle = 'white';
    ctx.font = '12px sans-serif';
    ctx.fillText("Calendar Render Error", canvas.width - 190, 20);
  }
}
