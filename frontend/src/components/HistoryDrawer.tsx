import { History, Eraser } from 'lucide-react';
import { 
  Drawer, 
  DrawerContent, 
  DrawerHeader, 
  DrawerTitle, 
  DrawerTrigger 
} from '@/components/ui/drawer';
import { Button } from '@/components/ui/button';
import { store } from '../../wailsjs/go/models';
import { HistoryItemCard } from './HistoryItemCard';

interface HistoryDrawerProps {
  history: store.HistoryItem[];
  monitors: any[];
  onApplyHistory: (item: store.HistoryItem, monitorId?: number) => void;
  onDeleteHistory: (key: string) => void;
  onClearHistory: () => void;
}

export function HistoryDrawer({ 
  history, 
  monitors,
  onApplyHistory, 
  onDeleteHistory, 
  onClearHistory 
}: HistoryDrawerProps) {
  // 后端 ListHistory 已按 CreatedAt 降序返回，直接使用；若需严格按日期展示可在此再排序
  const displayHistory = history;

  return (
    <Drawer shouldScaleBackground={false}>
      <DrawerTrigger asChild>
        <Button variant="ghost" size="icon" className="bg-black/40 backdrop-blur-md border border-white/10 rounded-full hover:bg-black/60 text-white">
          <History className="h-5 w-5" />
        </Button>
      </DrawerTrigger>
      <DrawerContent className="h-[85vh] antialiased">
        <DrawerHeader className="border-b shrink-0">
          <div className="flex items-center justify-between">
            <DrawerTitle>壁纸历史</DrawerTitle>
            <Button variant="ghost" size="sm" onClick={onClearHistory} className="text-destructive hover:text-destructive">
              <Eraser className="h-4 w-4 mr-2" />
              清空全部
            </Button>
          </div>
        </DrawerHeader>
        <div className="flex-1 min-h-0 overflow-y-auto">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 p-4 pb-12">
              {displayHistory.map((item) => (
                <HistoryItemCard 
                  key={item.key} 
                  item={item} 
                  monitors={monitors}
                  onApply={(monitorId) => onApplyHistory(item, monitorId)}
                  onDelete={() => onDeleteHistory(item.key)}
                />
              ))}
              {displayHistory.length === 0 && (
                <div className="col-span-full py-20 text-center text-muted-foreground font-light">
                  暂无历史记录
                </div>
              )}
            </div>
        </div>
      </DrawerContent>
    </Drawer>
  );
}
