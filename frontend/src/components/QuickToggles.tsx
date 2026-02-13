import { 
  Tooltip, 
  TooltipContent, 
  TooltipTrigger 
} from '@/components/ui/tooltip';
import { Switch } from '@/components/ui/switch';
import { Label } from '@/components/ui/label';
import { store } from '../../wailsjs/go/models';

interface QuickTogglesProps {
  config: store.Config | null;
  onConfigChange: (newConfig: store.Config) => void;
}

export function QuickToggles({ config, onConfigChange }: QuickTogglesProps) {
  if (!config) return null;

  return (
    <div className="absolute top-6 left-6 flex flex-col gap-4 z-10">
      <Tooltip>
        <TooltipTrigger asChild>
          <div className="flex items-center space-x-2 bg-black/40 backdrop-blur-md p-2 px-3 rounded-full border border-white/10 pointer-events-auto">
            <Switch 
              id="auto-apply" 
              checked={config.auto_apply || false}
              onCheckedChange={(val) => onConfigChange({ ...config, auto_apply: val })}
            />
            <Label htmlFor="auto-apply" className="text-xs font-light text-white cursor-pointer select-none">自动设置</Label>
          </div>
        </TooltipTrigger>
        <TooltipContent side="right">
          <p>获取新壁纸后自动设为桌面壁纸</p>
        </TooltipContent>
      </Tooltip>

      <Tooltip>
        <TooltipTrigger asChild>
          <div className="flex items-center space-x-2 bg-black/40 backdrop-blur-md p-2 px-3 rounded-full border border-white/10 pointer-events-auto">
            <Switch 
              id="overlay" 
              checked={config.overlay_metadata || false}
              onCheckedChange={(val) => onConfigChange({ ...config, overlay_metadata: val })}
            />
            <Label htmlFor="overlay" className="text-xs font-light text-white cursor-pointer select-none">叠加水印</Label>
          </div>
        </TooltipTrigger>
        <TooltipContent side="right">
          <p>在壁纸图片上叠加标题、日期及版权信息</p>
        </TooltipContent>
      </Tooltip>
    </div>
  );
}
