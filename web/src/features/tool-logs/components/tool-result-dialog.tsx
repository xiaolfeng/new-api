/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

interface ToolResultDialogProps {
  result: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ToolResultDialog(props: ToolResultDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const copied = copiedText === props.result

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Tool Result')}
      description={t('View the complete host tool result')}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      <div className='flex justify-end'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          onClick={() => void copyToClipboard(props.result)}
        >
          {copied ? <Check className='size-4' /> : <Copy className='size-4' />}
          <span className='ml-1.5'>{copied ? t('Copied') : t('Copy')}</span>
        </Button>
      </div>
      <ScrollArea className='max-h-[480px]'>
        <pre className='bg-muted/50 rounded-md border p-3 font-mono text-xs break-all whitespace-pre-wrap'>
          {props.result || t('No result')}
        </pre>
      </ScrollArea>
    </Dialog>
  )
}
