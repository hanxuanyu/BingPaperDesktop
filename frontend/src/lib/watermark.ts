import { Solar, Lunar, HolidayUtil } from 'lunar-javascript';

export async function renderWatermark(data: any): Promise<string> {
  const { 
    image_path, title, date, copyright, variant, 
    enable_watermark, enable_calendar, holiday_data,
    only_overlay, width, height, target_ratio
  } = data;

  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Could not get canvas context');

  let targetWidth = width;
  let targetHeight = height;
  let sourceImage: HTMLImageElement | null = null;

  const needSourceImage = Boolean(image_path) && (!only_overlay || !targetWidth || !targetHeight);
  if (needSourceImage) {
    const img = new Image();
    img.crossOrigin = "anonymous";
    img.src = image_path;
    
    await new Promise((resolve, reject) => {
      img.onload = resolve;
      img.onerror = reject;
    });

    sourceImage = img;
    if (!targetWidth) targetWidth = img.width;
    if (!targetHeight) targetHeight = img.height;
  }

  if (!targetWidth || !targetHeight) {
    throw new Error("Width and height required when no image_path provided");
  }

  canvas.width = targetWidth;
  canvas.height = targetHeight;

  if (!only_overlay) {
    if (!sourceImage) {
      throw new Error("image_path is required when rendering non-overlay output");
    }
    // Draw original image
    ctx.drawImage(sourceImage, 0, 0, targetWidth, targetHeight);
  }

  // 1. Draw Watermark if enabled
  if (enable_watermark) {
    drawWatermark(ctx, canvas, title, date, copyright, variant, target_ratio);
  }

  // 2. Draw Calendar if enabled
  if (enable_calendar) {
    drawCalendar(ctx, canvas, date, holiday_data, target_ratio);
  }

  return canvas.toDataURL(only_overlay ? 'image/png' : 'image/jpeg', only_overlay ? undefined : 0.95);
}

function calculateSafeArea(width: number, height: number, targetRatio?: number) {
  const currentRatio = width / height;
  const effectiveTargetRatio = targetRatio && targetRatio > 0 ? targetRatio : currentRatio;

  let visibleWidth = width;
  let visibleHeight = height;

  if (Math.abs(currentRatio - effectiveTargetRatio) > 0.01) {
    if (currentRatio > effectiveTargetRatio) {
      // 图片太宽，左右剪
      visibleWidth = height * effectiveTargetRatio;
    } else {
      // 图片太窄，上下剪
      visibleHeight = width / effectiveTargetRatio;
    }
  }

  const rightX = (width + visibleWidth) / 2;
  const leftX = (width - visibleWidth) / 2;
  const topY = (height - visibleHeight) / 2;
  const bottomY = (height + visibleHeight) / 2;

  // 基础边距：可见区域的 4%
  const paddingX = visibleWidth * 0.04;
  const paddingY = visibleHeight * 0.04;

  return { 
    right: rightX - paddingX, 
    left: leftX + paddingX,
    top: topY + paddingY, 
    bottom: bottomY - paddingY,
    visibleWidth,
    visibleHeight
  };
}

function drawWatermark(ctx: CanvasRenderingContext2D, canvas: HTMLCanvasElement, title: string, date: string, copyright: string, variant: string, targetRatio?: number) {
  const safeArea = calculateSafeArea(canvas.width, canvas.height, targetRatio);
  
  const titleFontSize = Math.max(24, Math.floor(safeArea.visibleHeight * 0.045));
  const copyrightFontSize = Math.max(14, Math.floor(safeArea.visibleHeight * 0.018));
  const tagFontSize = Math.max(12, Math.floor(safeArea.visibleHeight * 0.015));

  // Draw bottom gradient (可选，覆盖整个宽度还是仅可见区域？覆盖整个宽度比较保险)
  const gradient = ctx.createLinearGradient(0, canvas.height - safeArea.visibleHeight * 0.3, 0, canvas.height);
  gradient.addColorStop(0, 'rgba(0, 0, 0, 0)');
  gradient.addColorStop(1, 'rgba(0, 0, 0, 0.6)');
  ctx.fillStyle = gradient;
  ctx.fillRect(0, canvas.height - safeArea.visibleHeight * 0.3, canvas.width, safeArea.visibleHeight * 0.3);

  // Reset shadow for text
  ctx.shadowColor = 'rgba(0, 0, 0, 0.8)';
  ctx.shadowBlur = 12;
  ctx.shadowOffsetX = 2;
  ctx.shadowOffsetY = 2;

  // 计算位置（从下往上）
  const tagPaddingV = tagFontSize * 0.3;
  const tagY = safeArea.bottom;
  const tagRectHeight = tagFontSize + tagPaddingV * 2;
  
  const copyrightY = tagY - tagRectHeight - (tagFontSize * 1.2);
  const titleY = copyrightY - (copyrightFontSize * 1.8);
  
  const rightX = safeArea.right;

  ctx.save();
  ctx.textAlign = 'right';
  ctx.textBaseline = 'bottom';
  ctx.fillStyle = 'white';

  // Draw Title
  ctx.font = `bold ${titleFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.fillText(title, rightX, titleY);

  // Draw Copyright
  ctx.shadowBlur = 8;
  ctx.font = `${copyrightFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
  ctx.fillText(copyright, rightX, copyrightY);
  ctx.restore();

  // Draw Tags
  ctx.save();
  ctx.font = `${tagFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.textBaseline = 'bottom';
  
  const tags = [date, variant || "UHD"];
  const tagPaddingH = tagFontSize * 0.8;
  const tagRadius = 4;
  let currentTagX = rightX;

  // 靠右显示，所以倒序处理标签
  tags.slice().reverse().forEach(tag => {
    const tagWidth = ctx.measureText(tag).width;
    const rectWidth = tagWidth + tagPaddingH * 2;
    const rectX = currentTagX - rectWidth;
    const rectY = tagY - tagRectHeight;

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
      ctx.roundRect(rectX, rectY, rectWidth, tagRectHeight, tagRadius);
    } else {
      ctx.rect(rectX, rectY, rectWidth, tagRectHeight);
    }
    ctx.fill();
    ctx.stroke();
    ctx.restore();

    ctx.fillStyle = 'rgba(255, 255, 255, 0.6)';
    ctx.textAlign = 'left';
    ctx.fillText(tag, rectX + tagPaddingH, tagY - tagPaddingV);
    
    currentTagX -= (rectWidth + 12);
  });
  ctx.restore();
}

function drawCalendar(ctx: CanvasRenderingContext2D, canvas: HTMLCanvasElement, dateStr: string, holidayData: any[], targetRatio?: number) {
  try {
    // ... 解析日期代码保持不变 ...
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

    // 日历配置
    const safeArea = calculateSafeArea(canvas.width, canvas.height, targetRatio);
    const scale = safeArea.visibleHeight / 1080;

    const boxW = 320 * scale;
    const boxX = safeArea.right - boxW;
    const boxY = safeArea.top;

    // 0. 绘制右上角阴影增强对比度
    const topGradient = ctx.createLinearGradient(0, boxY, 0, boxY + 200 * scale);
    topGradient.addColorStop(0, 'rgba(0, 0, 0, 0.4)');
    topGradient.addColorStop(1, 'rgba(0, 0, 0, 0)');
    ctx.fillStyle = topGradient;
    ctx.fillRect(0, 0, canvas.width, boxY + 200 * scale);
    
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
