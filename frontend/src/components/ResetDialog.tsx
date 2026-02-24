import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';

interface ResetDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (onlySettings: boolean) => void;
}

export function ResetDialog({ open, onOpenChange, onConfirm }: ResetDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[425px]">
        <DialogHeader>
          <DialogTitle className="text-destructive">重置应用</DialogTitle>
        </DialogHeader>
        <div className="py-4 space-y-4">
          <div className="p-3 border rounded-lg hover:bg-muted/50 cursor-pointer transition-colors" onClick={() => onConfirm(true)}>
            <div className="font-medium text-sm">仅重置设置</div>
            <div className="text-xs text-muted-foreground mt-1">保留已下载的所有壁纸和历史记录，仅将应用配置恢复为默认。</div>
          </div>
          <div className="p-3 border border-destructive/20 rounded-lg hover:bg-destructive/5 cursor-pointer transition-colors" onClick={() => onConfirm(false)}>
            <div className="font-medium text-sm text-destructive">完全重置（数据+设置）</div>
            <div className="text-xs text-muted-foreground mt-1">物理删除所有壁纸、历史记录并将配置恢复为默认。此操作不可逆。</div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="ghost" className="w-full" onClick={() => onOpenChange(false)}>取消操作</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
