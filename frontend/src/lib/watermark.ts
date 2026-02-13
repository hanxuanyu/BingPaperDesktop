export async function renderWatermark(data: any): Promise<string> {
  const { image_path, title, date, copyright, variant } = data;
  const img = new Image();
  img.crossOrigin = "anonymous";
  img.src = image_path;
  
  await new Promise((resolve, reject) => {
    img.onload = resolve;
    img.onerror = reject;
  });

  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('Could not get canvas context');

  canvas.width = img.width;
  canvas.height = img.height;

  // Draw original image
  ctx.drawImage(img, 0, 0);

  // Watermark style
  const paddingX = canvas.width * 0.05;
  const paddingY = canvas.height * 0.05;
  const titleFontSize = Math.max(24, Math.floor(canvas.height * 0.045));
  const copyrightFontSize = Math.max(14, Math.floor(canvas.height * 0.018));
  const tagFontSize = Math.max(12, Math.floor(canvas.height * 0.015));

  // 1. Draw bottom gradient (simulating the UI's gradient)
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

  // 2. Draw Title (Bold)
  ctx.font = `bold ${titleFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.fillStyle = 'white';
  ctx.textBaseline = 'bottom';
  const titleY = canvas.height - paddingY - (copyrightFontSize * 2.5);
  ctx.fillText(title, paddingX, titleY);

  // 3. Draw Copyright
  ctx.shadowBlur = 8;
  ctx.font = `${copyrightFontSize}px "Segoe UI", Roboto, "Helvetica Neue", sans-serif`;
  ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
  const copyrightY = canvas.height - paddingY - (tagFontSize * 2.0);
  ctx.fillText(copyright, paddingX, copyrightY);

  // 4. Draw Date and Variant Tags
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

    // Draw tag background
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

    // Draw tag text
    ctx.fillStyle = 'rgba(255, 255, 255, 0.6)';
    ctx.fillText(tag, rectX + tagPaddingH, tagY - tagPaddingV);
    
    currentTagX += rectWidth + 12; // Gap
  });

  return canvas.toDataURL('image/jpeg', 0.95);
}
